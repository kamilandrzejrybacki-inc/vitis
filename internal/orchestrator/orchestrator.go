package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/terminal"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/util"
)

func Run(ctx context.Context, request model.RunRequest, deps Dependencies) (*model.RunResult, error) {
	if deps.Adapters == nil || deps.Runtime == nil || deps.Store == nil {
		return nil, &model.RunError{Code: model.ErrorInternal, Message: "missing orchestrator dependencies"}
	}
	if request.Provider == "" {
		request.Provider = "claude-code"
	}
	if request.TimeoutSec <= 0 {
		request.TimeoutSec = 600
	}
	if request.PeekLast <= 0 {
		request.PeekLast = 10
	}
	if request.TerminalCols <= 0 {
		request.TerminalCols = 80
	}
	if request.TerminalRows <= 0 {
		request.TerminalRows = 24
	}

	// 1. Resolve prompt.
	prompt, err := resolvePrompt(request)
	if err != nil {
		return nil, err
	}

	provider, err := deps.Adapters.Get(request.Provider)
	if err != nil {
		return nil, &model.RunError{Code: model.ErrorProvider, Message: err.Error()}
	}

	env, err := util.LoadEnvFile(request.EnvFile)
	if err != nil {
		return nil, &model.RunError{Code: model.ErrorConfig, Message: err.Error()}
	}
	if request.Model != "" {
		env["VITIS_MODEL"] = request.Model
	}
	if request.ReasoningEffort != "" {
		env["VITIS_REASONING_EFFORT"] = request.ReasoningEffort
	}

	homeDir := request.HomeDir
	if homeDir == "" {
		if currentHome, homeErr := os.UserHomeDir(); homeErr == nil {
			homeDir = currentHome
		}
	}

	// 2. Build session.
	session, spec, err := buildSession(provider, request, env, homeDir, prompt)
	if err != nil {
		return nil, &model.RunError{Code: model.ErrorInternal, Message: err.Error()}
	}

	// 3. Spawn PTY process (cleanup on failure deferred to after store create).
	process, err := deps.Runtime.Spawn(spec)
	if err != nil {
		return nil, &model.RunError{Code: model.ErrorSpawn, Message: err.Error()}
	}

	// 4. Create session in store (terminate process on failure).
	if err := deps.Store.CreateSession(ctx, session); err != nil {
		_ = process.Terminate(500)
		return nil, &model.RunError{Code: model.ErrorStore, Message: fmt.Errorf("create session: %w", err).Error()}
	}

	userTurn := model.Turn{
		SessionID: session.ID,
		Index:     0,
		Role:      "user",
		Content:   prompt,
		CreatedAt: time.Now().UTC(),
	}
	if err := deps.Store.AppendTurn(ctx, userTurn); err != nil {
		return nil, &model.RunError{Code: model.ErrorStore, Message: err.Error()}
	}

	if !spec.PromptInArgs {
		_, writeErr := process.Write(provider.FormatPrompt(prompt))
		if writeErr != nil {
			status := model.RunFailed
			endedAt := time.Now().UTC()
			duration := endedAt.Sub(session.StartedAt).Milliseconds()
			_ = deps.Store.UpdateSession(ctx, session.ID, model.SessionPatch{
				Status:     &status,
				EndedAt:    &endedAt,
				DurationMs: &duration,
			})
			return resultWithError(ctx, session.ID, request.Provider, model.RunFailed, request.PeekLast, deps, &model.RunError{
				Code:    model.ErrorPromptIO,
				Message: writeErr.Error(),
			})
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(request.TimeoutSec)*time.Second)
	defer cancel()

	// 5. Run completion loop.
	loopResult, err := waitForCompletionLoop(runCtx, session.ID, process, provider, deps.Store, request.DebugRaw)
	if err != nil {
		return nil, &model.RunError{Code: model.ErrorRuntime, Message: err.Error()}
	}

	rawTranscript := loopResult.Transcript.Raw()
	normalizedTranscript := loopResult.Transcript.Normalized()
	extraction := provider.ExtractResponse(rawTranscript, normalizedTranscript)

	if extraction.Response != "" {
		assistantTurn := model.Turn{
			SessionID: session.ID,
			Index:     1,
			Role:      "assistant",
			Content:   extraction.Response,
			CreatedAt: time.Now().UTC(),
		}
		if err := deps.Store.AppendTurn(ctx, assistantTurn); err != nil {
			return nil, &model.RunError{Code: model.ErrorStore, Message: err.Error()}
		}
	}

	// 6. Assemble result.
	result, patch := assembleResult(session, loopResult.Observation, extraction, loopResult.Transcript, loopResult.ExitCode)

	// 7. Update session in store.
	if err := deps.Store.UpdateSession(ctx, session.ID, patch); err != nil {
		return nil, &model.RunError{Code: model.ErrorStore, Message: err.Error()}
	}

	peek, err := deps.Store.PeekTurns(ctx, session.ID, request.PeekLast)
	if err != nil {
		return nil, &model.RunError{Code: model.ErrorStore, Message: err.Error()}
	}
	result.Peek = peek

	// 8. Return result.
	return result, nil
}

// buildSession constructs a model.Session and the adapter.SpawnSpec for a new run.
func buildSession(
	provider adapter.Adapter,
	request model.RunRequest,
	env map[string]string,
	homeDir string,
	prompt string,
) (model.Session, adapter.SpawnSpec, error) {
	sessionID, err := util.NewSessionID()
	if err != nil {
		return model.Session{}, adapter.SpawnSpec{}, err
	}
	startedAt := time.Now().UTC()
	session := model.Session{
		ID:           sessionID,
		Provider:     request.Provider,
		Status:       model.RunRunning,
		StartedAt:    startedAt,
		AuthMode:     "unknown",
		TerminalCols: intPtr(request.TerminalCols),
		TerminalRows: intPtr(request.TerminalRows),
	}
	spec := provider.BuildSpawnSpec(request.Cwd, env, homeDir, request.TerminalCols, request.TerminalRows, prompt)
	return session, spec, nil
}

// assembleResult builds the RunResult and the SessionPatch from the completed loop data.
func assembleResult(
	session model.Session,
	obs *adapter.TranscriptObservation,
	extraction adapter.ExtractionResult,
	transcript *terminal.Transcript,
	exitCode *int,
) (*model.RunResult, model.SessionPatch) {
	status := obs.Status
	endedAt := time.Now().UTC()
	duration := endedAt.Sub(session.StartedAt).Milliseconds()
	parserConfidence := extraction.ParserConfidence
	observationConfidence := obs.Confidence
	bytesCaptured := transcript.BytesSeen()
	blockedReason := obs.Reason

	patch := model.SessionPatch{
		Status:                &status,
		EndedAt:               &endedAt,
		DurationMs:            &duration,
		ExitCode:              exitCode,
		ParserConfidence:      &parserConfidence,
		ObservationConfidence: &observationConfidence,
		BytesCaptured:         &bytesCaptured,
		Warnings:              append([]string(nil), extraction.Notes...),
	}
	if status == model.RunBlockedOnInput || status == model.RunPermissionPrompt || status == model.RunAuthRequired || status == model.RunRateLimited {
		patch.BlockedReason = &blockedReason
	}

	result := &model.RunResult{
		SessionID: session.ID,
		Provider:  session.Provider,
		Status:    status,
		Response:  extraction.Response,
		Meta: model.ResultMeta{
			DurationMs:            duration,
			ExitCode:              exitCode,
			ParserConfidence:      extraction.ParserConfidence,
			ObservationConfidence: obs.Confidence,
			BytesCaptured:         bytesCaptured,
			Warnings:              append(obs.Evidence, extraction.Notes...),
			BlockedReason:         patch.BlockedReason,
		},
	}
	return result, patch
}

func resolvePrompt(request model.RunRequest) (string, error) {
	switch {
	case request.Prompt != "" && request.PromptFile != "":
		return "", &model.RunError{Code: model.ErrorConfig, Message: "exactly one of --prompt or --prompt-file must be set"}
	case request.Prompt != "":
		return request.Prompt, nil
	case request.PromptFile != "":
		if strings.Contains(filepath.Clean(request.PromptFile), "..") {
			return "", fmt.Errorf("prompt-file path must not contain '..': %s", request.PromptFile)
		}
		data, err := os.ReadFile(request.PromptFile)
		if err != nil {
			return "", &model.RunError{Code: model.ErrorInput, Message: fmt.Sprintf("read prompt file: %v", err)}
		}
		return string(data), nil
	default:
		return "", &model.RunError{Code: model.ErrorConfig, Message: "missing prompt input"}
	}
}

func resultWithError(ctx context.Context, sessionID, provider string, status model.RunStatus, peekLast int, deps Dependencies, runErr *model.RunError) (*model.RunResult, error) {
	peek, err := deps.Store.PeekTurns(ctx, sessionID, peekLast)
	if err != nil {
		return nil, err
	}
	return &model.RunResult{
		SessionID: sessionID,
		Provider:  provider,
		Status:    status,
		Response:  "",
		Peek:      peek,
		Meta: model.ResultMeta{
			DurationMs:            0,
			ParserConfidence:      0,
			ObservationConfidence: 0,
			BytesCaptured:         0,
		},
		Error: runErr,
	}, nil
}

func intPtr(value int) *int { return &value }
