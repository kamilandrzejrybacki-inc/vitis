package terminal

import (
	"bytes"
	"sync"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

type Transcript struct {
	mu            sync.Mutex
	fullRaw       bytes.Buffer
	tail          []byte
	tailCapacity  int
	lastOutputAt  time.Time
	firstOutputAt time.Time
	exitCode      *int
	bytesSeen     int64
}

func NewTranscript(tailCapacity int) *Transcript {
	if tailCapacity <= 0 {
		tailCapacity = 64 * 1024
	}
	return &Transcript{
		tailCapacity: tailCapacity,
	}
}

func (t *Transcript) Append(event model.StreamEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(event.Data) == 0 {
		return
	}

	if t.firstOutputAt.IsZero() {
		t.firstOutputAt = event.Timestamp
	}
	t.lastOutputAt = event.Timestamp
	t.bytesSeen += int64(len(event.Data))
	_, _ = t.fullRaw.Write(event.Data)

	t.tail = append(t.tail, event.Data...)
	if len(t.tail) > t.tailCapacity {
		fresh := make([]byte, t.tailCapacity)
		copy(fresh, t.tail[len(t.tail)-t.tailCapacity:])
		t.tail = fresh
	}
}

func (t *Transcript) RecordExit(code int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.exitCode = &code
}

func (t *Transcript) ExitCode() *int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.exitCode == nil {
		return nil
	}
	code := *t.exitCode
	return &code
}

func (t *Transcript) Raw() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]byte(nil), t.fullRaw.Bytes()...)
}

func (t *Transcript) TailRaw() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]byte(nil), t.tail...)
}

func (t *Transcript) Normalized() string {
	return NormalizePTYText(t.Raw())
}

func (t *Transcript) TailNormalized() string {
	return NormalizePTYText(t.TailRaw())
}

func (t *Transcript) BytesSeen() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.bytesSeen
}

func (t *Transcript) IdleSince(now time.Time) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastOutputAt.IsZero() {
		return 0
	}
	return now.Sub(t.lastOutputAt)
}

func (t *Transcript) HasOutput() bool {
	return t.BytesSeen() > 0
}
