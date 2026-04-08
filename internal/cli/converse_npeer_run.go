package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/conversation"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/peer"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/peer/provider"
	filestore "github.com/kamilandrzejrybacki-inc/vitis/internal/store/file"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/terminator"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/util"
)

// runNPeerConverse executes a conversation in N-peer mode (--peer flags).
// It mirrors the validation, wiring, and streaming logic of the legacy
// 2-peer path in ConverseCommand, but builds a v2 model.Conversation and
// uses the broker's PeersByID/PeerOrder transport surface.
//
// Exit codes match the legacy path:
//
//	0 — conversation reached a terminal status
//	1 — runtime error (peer crash, spawn failure, bus error)
//	2 — configuration error
func runNPeerConverse(
	ctx context.Context,
	rawPeers []string,
	broadcastSeed, opener string,
	maxTurns int,
	terminatorKind, sentinelTok string,
	perTurnTimeout, overallTimeout int,
	busKind, logBackend, logPath, workingDir string,
	streamTurns bool,
	replyStyle string,
	stdout, stderr io.Writer,
) int {
	// Parse + validate the N-peer config.
	cfg, err := parseNPeerSpecs(rawPeers, broadcastSeed, opener)
	if err != nil {
		fmt.Fprintf(stderr, "converse: %v\n", err)
		return 2
	}

	// Validate the shared knobs.
	if terminatorKind != "sentinel" {
		fmt.Fprintf(stderr, "converse: terminator %q not supported in this build (sentinel only)\n", terminatorKind)
		return 2
	}
	if busKind != "inproc" {
		fmt.Fprintf(stderr, "converse: bus %q not supported in this build (inproc only)\n", busKind)
		return 2
	}
	if logBackend != "file" {
		fmt.Fprintf(stderr, "converse: log-backend %q not supported in this build (file only)\n", logBackend)
		return 2
	}
	if !conversation.IsValidStyle(replyStyle) {
		fmt.Fprintf(stderr, "converse: --style %q is not valid (use normal, caveman-lite, caveman-full, or caveman-ultra)\n", replyStyle)
		return 2
	}
	if maxTurns < 1 || maxTurns > 500 {
		fmt.Fprintln(stderr, "converse: --max-turns must be in [1,500]")
		return 2
	}
	if perTurnTimeout < 1 || perTurnTimeout > 3600 {
		fmt.Fprintln(stderr, "converse: --per-turn-timeout must be in [1,3600]")
		return 2
	}
	if overallTimeout > 86400 {
		fmt.Fprintln(stderr, "converse: --overall-timeout must be at most 86400 seconds (24h)")
		return 2
	}
	if overallTimeout == 0 {
		if maxTurns > 0 && perTurnTimeout > 0 && perTurnTimeout > math.MaxInt/maxTurns {
			fmt.Fprintln(stderr, "converse: --max-turns * --per-turn-timeout would overflow; set --overall-timeout explicitly")
			return 2
		}
		overallTimeout = maxTurns * perTurnTimeout
	}

	// Sanitise paths.
	logPath = filepath.Clean(logPath)
	if workingDir != "" {
		workingDir = filepath.Clean(workingDir)
		if fi, err := os.Stat(workingDir); err != nil {
			fmt.Fprintf(stderr, "converse: --working-directory %q: %v\n", workingDir, err)
			return 2
		} else if !fi.IsDir() {
			fmt.Fprintf(stderr, "converse: --working-directory %q is not a directory\n", workingDir)
			return 2
		}
	}

	// Build the v2 conversation model.
	peers, seeds := cfg.toV2Conversation()
	// Inject working_directory into each peer's options if not already set.
	if workingDir != "" {
		for i := range peers {
			if peers[i].Spec.Options == nil {
				peers[i].Spec.Options = map[string]string{}
			}
			if peers[i].Spec.Options["cwd"] == "" {
				peers[i].Spec.Options["cwd"] = workingDir
			}
		}
	}

	conv := model.Conversation{
		ID:             util.NewID("conv_"),
		SchemaVersion:  2,
		CreatedAt:      time.Now().UTC(),
		Status:         model.ConvRunning,
		MaxTurns:       maxTurns,
		PerTurnTimeout: int64(perTurnTimeout),
		OverallTimeout: int64(overallTimeout),
		Terminator:     model.TerminatorSpec{Kind: "sentinel", Sentinel: sentinelTok},
		Peers:          peers,
		Seeds:          seeds,
		OpenerID:       cfg.OpenerID,
		ReplyStyle:     replyStyle,
	}

	// Wire dependencies.
	store, err := filestore.New(logPath, false)
	if err != nil {
		fmt.Fprintf(stderr, "converse: store init: %v\n", err)
		return 1
	}
	defer store.Close()

	b := inproc.New()
	defer b.Close()

	// Build one PeerTransport per declared peer using the existing
	// provider factory. The transport map is keyed by peer id.
	spawner := provider.NewTerminalSpawner()
	peersByID := make(map[model.PeerID]peer.PeerTransport, len(peers))
	peerOrder := make([]model.PeerID, 0, len(peers))
	for _, p := range peers {
		peersByID[p.ID] = provider.New(spawner, conv.PerTurnTimeoutDuration())
		peerOrder = append(peerOrder, p.ID)
	}

	term := terminator.NewSentinel(sentinelTok)

	deps := conversation.BrokerDeps{
		Conversation: conv,
		Terminator:   term,
		Bus:          b,
		Store:        store,
		PeersByID:    peersByID,
		PeerOrder:    peerOrder,
	}
	br := conversation.NewBroker(deps)

	runCtx, runCancel := context.WithTimeout(ctx, conv.OverallTimeoutDuration())
	defer runCancel()

	var streamWg sync.WaitGroup
	if streamTurns {
		streamWg.Add(1)
		go func() {
			defer streamWg.Done()
			streamTurnsTo(runCtx, b, conv.ID, stdout)
		}()
	}

	res, err := br.Run(runCtx)
	_ = b.Close()
	streamWg.Wait()
	runCancel()

	if err != nil {
		fmt.Fprintf(stderr, "converse: broker error: %v\n", err)
		return 1
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if encErr := enc.Encode(res); encErr != nil {
		fmt.Fprintf(stderr, "converse: encode result: %v\n", encErr)
		return 1
	}

	switch res.Conversation.Status {
	case model.ConvCompletedSentinel, model.ConvCompletedJudge, model.ConvMaxTurnsHit, model.ConvInterrupted:
		return 0
	default:
		return 1
	}
}
