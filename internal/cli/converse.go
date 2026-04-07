package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus"
	"github.com/kamilandrzejrybacki-inc/clank/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/clank/internal/conversation"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
	"github.com/kamilandrzejrybacki-inc/clank/internal/peer/provider"
	filestore "github.com/kamilandrzejrybacki-inc/clank/internal/store/file"
	"github.com/kamilandrzejrybacki-inc/clank/internal/terminator"
	"github.com/kamilandrzejrybacki-inc/clank/internal/util"

	"flag"
)

// repeatableFlag implements flag.Value for --peer-a-opt and --peer-b-opt.
type repeatableFlag struct {
	values map[string]string
}

func newRepeatableFlag() *repeatableFlag { return &repeatableFlag{values: map[string]string{}} }

func (r *repeatableFlag) String() string {
	if r == nil || len(r.values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(r.values))
	for k, v := range r.values {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ",")
}

func (r *repeatableFlag) Set(value string) error {
	idx := strings.Index(value, "=")
	if idx <= 0 {
		return fmt.Errorf("expected key=value, got %q", value)
	}
	r.values[value[:idx]] = value[idx+1:]
	return nil
}

// ConverseCommand parses arguments, validates them, runs the conversation,
// and writes the FinalResult as JSON to stdout. Diagnostic messages go to
// stderr. Returns:
//
//	0  - conversation reached a terminal status
//	1  - runtime error (peer crash, spawn failure, bus error)
//	2  - configuration error
func ConverseCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("converse", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		peerA          = fs.String("peer-a", "", "peer A URI (provider:<id>)")
		peerB          = fs.String("peer-b", "", "peer B URI (provider:<id>)")
		seed           = fs.String("seed", "", "seed text for both peers")
		seedA          = fs.String("seed-a", "", "asymmetric seed for peer A")
		seedB          = fs.String("seed-b", "", "asymmetric seed for peer B")
		opener         = fs.String("opener", "a", "which peer opens the conversation: a or b")
		maxTurns       = fs.Int("max-turns", 50, "maximum total turns (1..500)")
		terminatorKind = fs.String("terminator", "sentinel", "termination strategy: sentinel (judge in plan 3)")
		sentinelTok    = fs.String("sentinel", "<<END>>", "sentinel token for sentinel terminator")
		perTurnTimeout = fs.Int("per-turn-timeout", 300, "per-turn timeout in seconds")
		overallTimeout = fs.Int("overall-timeout", 0, "overall timeout in seconds; defaults to max-turns*per-turn-timeout")
		busKind        = fs.String("bus", "inproc", "bus backend: inproc")
		logBackend     = fs.String("log-backend", "file", "log backend: file")
		logPath        = fs.String("log-path", "./logs", "file backend log root")
		workingDir     = fs.String("working-directory", "", "working directory for spawned peers")
		streamTurns    = fs.Bool("stream-turns", true, "stream each turn as JSONL on stdout during the run")
	)
	peerAOpts := newRepeatableFlag()
	peerBOpts := newRepeatableFlag()
	fs.Var(peerAOpts, "peer-a-opt", "peer A option (repeatable, key=value)")
	fs.Var(peerBOpts, "peer-b-opt", "peer B option (repeatable, key=value)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Validation
	if *peerA == "" || *peerB == "" {
		fmt.Fprintln(stderr, "converse: --peer-a and --peer-b are required")
		return 2
	}
	if *seed == "" && (*seedA == "" || *seedB == "") {
		fmt.Fprintln(stderr, "converse: provide --seed or both --seed-a and --seed-b")
		return 2
	}
	if *seed != "" && (*seedA != "" || *seedB != "") {
		fmt.Fprintln(stderr, "converse: --seed is mutually exclusive with --seed-a/--seed-b")
		return 2
	}
	if *opener != "a" && *opener != "b" {
		fmt.Fprintln(stderr, "converse: --opener must be a or b")
		return 2
	}
	if *maxTurns < 1 || *maxTurns > 500 {
		fmt.Fprintln(stderr, "converse: --max-turns must be in [1,500]")
		return 2
	}
	if *terminatorKind != "sentinel" {
		fmt.Fprintf(stderr, "converse: terminator %q not supported in this build (sentinel only)\n", *terminatorKind)
		return 2
	}
	if *busKind != "inproc" {
		fmt.Fprintf(stderr, "converse: bus %q not supported in this build (inproc only)\n", *busKind)
		return 2
	}
	if *logBackend != "file" {
		fmt.Fprintf(stderr, "converse: log-backend %q not supported in this build (file only)\n", *logBackend)
		return 2
	}
	if *perTurnTimeout < 1 {
		fmt.Fprintln(stderr, "converse: --per-turn-timeout must be positive")
		return 2
	}
	if *overallTimeout == 0 {
		*overallTimeout = *maxTurns * *perTurnTimeout
	}

	// Sanitise file-system paths to prevent relative escape sequences.
	*logPath = filepath.Clean(*logPath)
	if *workingDir != "" {
		*workingDir = filepath.Clean(*workingDir)
		if fi, err := os.Stat(*workingDir); err != nil {
			fmt.Fprintf(stderr, "converse: --working-directory %q: %v\n", *workingDir, err)
			return 2
		} else if !fi.IsDir() {
			fmt.Fprintf(stderr, "converse: --working-directory %q is not a directory\n", *workingDir)
			return 2
		}
	}

	conv := model.Conversation{
		ID:             util.NewID("conv_"),
		CreatedAt:      time.Now().UTC(),
		Status:         model.ConvRunning,
		MaxTurns:       *maxTurns,
		PerTurnTimeout: int64(*perTurnTimeout),
		OverallTimeout: int64(*overallTimeout),
		Terminator:     model.TerminatorSpec{Kind: "sentinel", Sentinel: *sentinelTok},
		PeerA:          model.PeerSpec{URI: *peerA, Options: mergeOptions(peerAOpts.values, *workingDir)},
		PeerB:          model.PeerSpec{URI: *peerB, Options: mergeOptions(peerBOpts.values, *workingDir)},
		SeedA:          converseFirstNonEmpty(*seedA, *seed),
		SeedB:          converseFirstNonEmpty(*seedB, *seed),
		Opener:         model.PeerSlot(*opener),
	}

	store, err := filestore.New(*logPath, false)
	if err != nil {
		fmt.Fprintf(stderr, "converse: store init: %v\n", err)
		return 1
	}
	defer store.Close()

	b := inproc.New()
	defer b.Close()

	spawner := provider.NewTerminalSpawner()
	pa := provider.New(spawner, conv.PerTurnTimeoutDuration())
	pb := provider.New(spawner, conv.PerTurnTimeoutDuration())
	term := terminator.NewSentinel(*sentinelTok)

	deps := conversation.BrokerDeps{
		Conversation: conv,
		PeerA:        pa,
		PeerB:        pb,
		Terminator:   term,
		Bus:          b,
		Store:        store,
	}
	br := conversation.NewBroker(deps)

	runCtx, runCancel := context.WithTimeout(ctx, conv.OverallTimeoutDuration())
	defer runCancel()

	var streamWg sync.WaitGroup
	if *streamTurns {
		streamWg.Add(1)
		go func() {
			defer streamWg.Done()
			streamTurnsTo(runCtx, b, conv.ID, stdout)
		}()
	}

	res, err := br.Run(runCtx)
	// Cancel runCtx explicitly so the stream goroutine exits promptly, then
	// wait for it to finish before writing the final result to stdout.
	runCancel()
	streamWg.Wait()

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

func mergeOptions(in map[string]string, workingDir string) map[string]string {
	out := make(map[string]string, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	if workingDir != "" && out["cwd"] == "" {
		out["cwd"] = workingDir
	}
	return out
}

func converseFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func streamTurnsTo(ctx context.Context, b bus.Bus, conversationID string, w io.Writer) {
	sub, cancel, err := b.Subscribe(ctx, bus.TopicTurn(conversationID))
	if err != nil {
		return
	}
	defer cancel()
	enc := json.NewEncoder(w)
	for {
		select {
		case <-ctx.Done():
			return
		case msg, open := <-sub:
			if !open {
				return
			}
			var turn model.ConversationTurn
			if uerr := json.Unmarshal(msg.Payload, &turn); uerr != nil {
				continue
			}
			_ = enc.Encode(turn)
		}
	}
}
