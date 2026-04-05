package adapter

import "github.com/kamilandrzejrybacki-inc/clank/internal/model"

type SpawnSpec struct {
	Command      string
	Args         []string
	Env          map[string]string
	Cwd          string
	HomeDir      string
	TerminalCols int
	TerminalRows int
}

type TranscriptObservation struct {
	Status     model.RunStatus
	Terminal   bool
	Confidence float64
	Reason     string
	Evidence   []string
}

type ExtractionResult struct {
	Response         string
	ParserConfidence float64
	Notes            []string
}

type CompletionContext struct {
	RawTail        []byte
	NormalizedTail string
	ElapsedMs      int64
	IdleMs         int64
	ExitCode       *int
	BytesSeen      int64
	LastWriteMs    int64
}

type Adapter interface {
	ID() string
	BuildSpawnSpec(cwd string, env map[string]string, homeDir string, cols, rows int) SpawnSpec
	FormatPrompt(raw string) []byte
	Observe(context CompletionContext) *TranscriptObservation
	ExtractResponse(rawTranscript []byte, normalizedTranscript string) ExtractionResult
}
