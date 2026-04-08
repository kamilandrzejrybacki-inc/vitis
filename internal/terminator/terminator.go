// Package terminator defines the Terminator interface and ships built-in
// implementations. Terminators run as bus subscribers, watching the turn
// topic for a configured signal and publishing a Verdict (wrapped in a
// ControlMsg) to the control topic when they decide a conversation should
// end. Terminators do not interact with peers directly.
package terminator

import (
	"context"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/bus"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

// Terminator is the interface implemented by sentinel and judge strategies.
type Terminator interface {
	Start(ctx context.Context, conv model.Conversation, b bus.Bus) error
	Stop(ctx context.Context) error
}
