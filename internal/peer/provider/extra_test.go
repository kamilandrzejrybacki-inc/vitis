package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kamilandrzejrybacki-inc/clank/internal/bus/inproc"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

func TestBuildPeerEnv_AllowedAndModel(t *testing.T) {
	env := buildPeerEnv(map[string]string{
		"env_ANTHROPIC_API_KEY": "secret",
		"env_LD_PRELOAD":        "evil.so",
		"model":                 "claude-3-opus",
		"reasoning-effort":      "high",
		"unknown":               "ignored",
	})
	if env["ANTHROPIC_API_KEY"] != "secret" {
		t.Errorf("expected allowed key forwarded; got %v", env)
	}
	if _, has := env["LD_PRELOAD"]; has {
		t.Errorf("disallowed key must be dropped; got %v", env)
	}
	if env["CLANK_MODEL"] != "claude-3-opus" {
		t.Errorf("model not mapped: %v", env)
	}
	if env["CLANK_REASONING_EFFORT"] != "high" {
		t.Errorf("reasoning-effort not mapped: %v", env)
	}
}

func TestBuildPeerEnv_Empty(t *testing.T) {
	env := buildPeerEnv(map[string]string{})
	if len(env) != 0 {
		t.Errorf("expected empty env, got %v", env)
	}
}

func TestBuildPersistentSpawnSpec_BadScheme(t *testing.T) {
	_, err := buildPersistentSpawnSpec(model.PeerSpec{URI: "http://x"})
	if err == nil {
		t.Fatal("expected error for non-provider scheme")
	}
}

func TestBuildPersistentSpawnSpec_UnknownProvider(t *testing.T) {
	_, err := buildPersistentSpawnSpec(model.PeerSpec{URI: "provider:nope"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestBuildPersistentSpawnSpec_Codex(t *testing.T) {
	spec, err := buildPersistentSpawnSpec(model.PeerSpec{
		URI: "provider:codex",
		Options: map[string]string{
			"model":            "gpt-5",
			"reasoning-effort": "medium",
		},
	})
	require.NoError(t, err)
	if spec.PromptInArgs {
		t.Error("PromptInArgs must be false")
	}
	args := strings.Join(spec.Args, " ")
	if !strings.Contains(args, "--model gpt-5") || !strings.Contains(args, "--reasoning-effort medium") {
		t.Errorf("expected model+effort in args, got %v", spec.Args)
	}
}

func TestBuildPersistentSpawnSpec_ClaudeCode(t *testing.T) {
	spec, err := buildPersistentSpawnSpec(model.PeerSpec{URI: "provider:claude-code"})
	require.NoError(t, err)
	if spec.Command == "" {
		t.Error("expected non-empty command for claude-code")
	}
}

func TestResolveAdapter_Cases(t *testing.T) {
	if _, err := resolveAdapter(model.PeerSpec{URI: "bad"}); err == nil {
		t.Error("expected error for bad scheme")
	}
	if _, err := resolveAdapter(model.PeerSpec{URI: "provider:claude-code"}); err != nil {
		t.Errorf("claude-code: %v", err)
	}
	if _, err := resolveAdapter(model.PeerSpec{URI: "provider:codex"}); err != nil {
		t.Errorf("codex: %v", err)
	}
	if _, err := resolveAdapter(model.PeerSpec{URI: "provider:nope"}); err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestTransport_StopBeforeStart(t *testing.T) {
	tx := New(nil, time.Second)
	if err := tx.Stop(context.Background(), 10*time.Millisecond); err != nil {
		t.Errorf("Stop before Start should be no-op: %v", err)
	}
}

func TestTransport_StopIdempotent(t *testing.T) {
	pty := newFakePTY()
	tx := New(func(_ context.Context, _ model.PeerSpec) (rawPTYProcess, error) {
		return pty, nil
	}, time.Second)
	bus := inproc.New()
	defer bus.Close()
	require.NoError(t, tx.Start(context.Background(), model.PeerSpec{URI: "provider:fake"}, bus, "c", model.PeerSlotA))
	require.NoError(t, tx.Stop(context.Background(), 10*time.Millisecond))
	require.NoError(t, tx.Stop(context.Background(), 10*time.Millisecond))
}

func TestTransport_DoubleStart(t *testing.T) {
	pty := newFakePTY()
	tx := New(func(_ context.Context, _ model.PeerSpec) (rawPTYProcess, error) {
		return pty, nil
	}, time.Second)
	bus := inproc.New()
	defer bus.Close()
	ctx := context.Background()
	require.NoError(t, tx.Start(ctx, model.PeerSpec{URI: "provider:fake"}, bus, "c", model.PeerSlotA))
	err := tx.Start(ctx, model.PeerSpec{URI: "provider:fake"}, bus, "c", model.PeerSlotA)
	if err == nil {
		t.Error("expected error on double start")
	}
	_ = tx.Stop(ctx, 10*time.Millisecond)
}

func TestTransport_DeliverBeforeStart(t *testing.T) {
	tx := New(nil, time.Second)
	_, err := tx.Deliver(context.Background(), model.Envelope{Body: "x", MarkerToken: "M"})
	if err == nil {
		t.Error("expected error on deliver before start")
	}
}

func TestNew_ZeroTimeoutDefaults(t *testing.T) {
	tx := New(nil, 0)
	if tx.perTurnTimeout <= 0 {
		t.Errorf("expected positive default timeout, got %v", tx.perTurnTimeout)
	}
}

func TestPersistentProcess_EmptyMarker(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	defer pp.Close(0)
	_, err := pp.ConverseTurn(context.Background(), []byte("x"), "", time.Second)
	if err == nil {
		t.Error("expected error on empty marker")
	}
}

func TestPersistentProcess_DrainAvailable(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	defer pp.Close(0)
	pty.emit("some data")
	// Wait for pump to absorb.
	time.Sleep(20 * time.Millisecond)
	got := pp.drainAvailable()
	if string(got) != "some data" {
		t.Errorf("expected drained 'some data', got %q", got)
	}
	// Second drain returns nil.
	if pp.drainAvailable() != nil {
		t.Error("second drain should return nil")
	}
}

func TestPersistentProcess_CloseIdempotent(t *testing.T) {
	pty := newFakePTY()
	pp := NewPersistentProcess(pty)
	if err := pp.Close(10 * time.Millisecond); err != nil {
		t.Errorf("first close: %v", err)
	}
	if err := pp.Close(10 * time.Millisecond); err != nil {
		t.Errorf("second close: %v", err)
	}
}
