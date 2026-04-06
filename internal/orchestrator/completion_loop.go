package orchestrator

import (
	"context"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
	"github.com/kamilandrzejrybacki-inc/clank/internal/store"
	"github.com/kamilandrzejrybacki-inc/clank/internal/terminal"
)

type completionResult struct {
	Observation *adapter.TranscriptObservation
	Transcript  *terminal.Transcript
	ExitCode    *int
}

func waitForCompletionLoop(
	ctx context.Context,
	sessionID string,
	process terminal.PseudoTerminalProcess,
	provider adapter.Adapter,
	store store.Store,
	debugRaw bool,
) (*completionResult, error) {
	transcript := terminal.NewTranscript(64 * 1024)
	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var exitCode *int
	outputCh := process.Output()

	for {
		select {
		case <-ctx.Done():
			_ = process.Terminate(5000)
			observation := &adapter.TranscriptObservation{
				Status:     model.RunTimeout,
				Terminal:   true,
				Confidence: 1.0,
				Reason:     "context deadline exceeded",
				Evidence:   []string{"timeout"},
			}
			return &completionResult{Observation: observation, Transcript: transcript, ExitCode: exitCode}, nil
		case event, ok := <-outputCh:
			if !ok {
				outputCh = nil
				continue
			}
			transcript.Append(event)
			if debugRaw {
				_ = store.AppendStreamEvent(ctx, model.StoredStreamEvent{
					SessionID: sessionID,
					Timestamp: event.Timestamp,
					Kind:      event.Kind,
					Data:      event.Data,
				})
			}
		case done, ok := <-process.Done():
			if !ok {
				done = model.ExitResult{Code: 0}
			}
			exitCode = &done.Code
			transcript.RecordExit(done.Code)
		case <-ticker.C:
			observation := provider.Observe(adapter.CompletionContext{
				RawTail:        transcript.TailRaw(),
				NormalizedTail: transcript.TailNormalized(),
				ElapsedMs:      time.Since(start).Milliseconds(),
				IdleMs:         transcript.IdleSince(time.Now()).Milliseconds(),
				ExitCode:       exitCode,
				BytesSeen:      transcript.BytesSeen(),
			})
			if observation != nil && observation.Terminal {
				return &completionResult{
					Observation: observation,
					Transcript:  transcript,
					ExitCode:    exitCode,
				}, nil
			}
		}
	}
}
