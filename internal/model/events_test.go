package model

import "testing"

func TestStreamEventKind_Constants(t *testing.T) {
	if StreamEventInput == "" {
		t.Error("StreamEventInput must not be empty")
	}
	if StreamEventOutput == "" {
		t.Error("StreamEventOutput must not be empty")
	}
	if StreamEventInput == StreamEventOutput {
		t.Errorf("StreamEventInput and StreamEventOutput must be distinct, both are %q", StreamEventInput)
	}
}

func TestExitResult_ZeroValue(t *testing.T) {
	var r ExitResult
	if r.Code != 0 {
		t.Errorf("expected zero Code, got %d", r.Code)
	}
	if r.Err != nil {
		t.Errorf("expected nil Err, got %v", r.Err)
	}
}
