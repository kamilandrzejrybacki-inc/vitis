package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
	"github.com/kamilandrzejrybacki-inc/clank/internal/util"
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

	homeDir := request.HomeDir
	if homeDir == "" {
		if currentHome, homeErr := os.UserHomeDir(); homeErr == nil {
			homeDir = currentHome
		}
	}

	sessionID, err := util.NewSessionID()
	if err != nil {
		return nil, &model.RunError{Code: model.ErrorInternal, Message: err.Error()}
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
	spec := provider.BuildSpawnSpec(request.Cwd, env, homeDir, request.TerminalCols, request.TerminalRows)
	process, err := deps.Runtime.Spawn(spec)
	if err != nil {
		return nil, &model.RunError{Code: model.ErrorSpawn, Message: err.Error()}
	}

	if err := deps.Store.CreateSession(ctx, session); err != nil {
		_ = process.Terminate(500)
		return nil, &model.RunError{Code: model.ErrorStore, Message: fmt.Errorf("create session: %w", err).Error()}
	}

	userTurn := model.Turn{
		SessionID: sessionID,
		Index:     0,
		Role:      "user",
		Content:   prompt,
		CreatedAt: time.Now().UTC(),
	}
	if err := deps.Store.AppendTurn(ctx, userTurn); err != nil {
		return nil, &model.RunError{Code: model.ErrorStore, Message: err.Error()}
	}

	_, writeErr := process.Write(provider.FormatPrompt(prompt))
	if writeErr != nil {
		status := model.RunFailed
		endedAt := time.Now().UTC()
		duration := endedAt.Sub(startedAt).Milliseconds()
		_ = deps.Store.UpdateSession(ctx, sessionID, model.SessionPatch{
			Status:     &status,
			EndedAt:    &endedAt,
			DurationMs: &duration,
		})
		return resultWithError(ctx, sessionID, request.Provider, model.RunFailed, request.PeekLast, deps, &model.RunError{
			Code:    model.ErrorPromptIO,
			Message: writeErr.Error(),
		})
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(request.TimeoutSec)*time.Second)
	defer cancel()

	loopResult, err := waitForCompletionLoop(runCtx, sessionID, process, provider, deps.Store, request.DebugRaw)
	if err != nil {
		return nil, &model.RunError{Code: model.ErrorRuntime, Message: err.Error()}
	}

	rawTranscript := loopResult.Transcript.Raw()
	normalizedTranscript := loopResult.Transcript.Normalized()
	extraction := provider.ExtractResponse(rawTranscript, normalizedTranscript)

	if extraction.Response != "" {
		assistantTurn := model.Turn{
			SessionID: sessionID,
			Index:     1,
			Role:      "assistant",
			Content:   extraction.Response,
			CreatedAt: time.Now().UTC(),
		}
		if err := deps.Store.AppendTurn(ctx, assistantTurn); err != nil {
			return nil, &model.RunError{Code: model.ErrorStore, Message: err.Error()}
		}
	}

	status := loopResult.Observation.Status
	endedAt := time.Now().UTC()
	duration := endedAt.Sub(startedAt).Milliseconds()
	parserConfidence := extraction.ParserConfidence
	observationConfidence := loopResult.Observation.Confidence
	bytesCaptured := loopResult.Transcript.BytesSeen()
	blockedReason := loopResult.Observation.Reason
	patch := model.SessionPatch{
		Status:                &status,
		EndedAt:               &endedAt,
		DurationMs:            &duration,
		ExitCode:              loopResult.ExitCode,
		ParserConfidence:      &parserConfidence,
		ObservationConfidence: &observationConfidence,
		BytesCaptured:         &bytesCaptured,
		Warnings:              append([]string(nil), extraction.Notes...),
	}
	if status == model.RunBlockedOnInput || status == model.RunPermissionPrompt || status == model.RunAuthRequired || status == model.RunRateLimited {
		patch.BlockedReason = &blockedReason
	}
	if err := deps.Store.UpdateSession(ctx, sessionID, patch); err != nil {
		return nil, &model.RunError{Code: model.ErrorStore, Message: err.Error()}
	}

	peek, err := deps.Store.PeekTurns(ctx, sessionID, request.PeekLast)
	if err != nil {
		return nil, &model.RunError{Code: model.ErrorStore, Message: err.Error()}
	}

	return &model.RunResult{
		SessionID: sessionID,
		Provider:  request.Provider,
		Status:    status,
		Response:  extraction.Response,
		Peek:      peek,
		Meta: model.ResultMeta{
			DurationMs:            duration,
			ExitCode:              loopResult.ExitCode,
			ParserConfidence:      extraction.ParserConfidence,
			ObservationConfidence: loopResult.Observation.Confidence,
			BytesCaptured:         bytesCaptured,
			Warnings:              append(loopResult.Observation.Evidence, extraction.Notes...),
			BlockedReason:         patch.BlockedReason,
		},
	}, nil
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
