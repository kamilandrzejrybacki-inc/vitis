package orchestrator

import (
	"github.com/kamilandrzejrybacki-inc/vitis/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/store"
	"github.com/kamilandrzejrybacki-inc/vitis/internal/terminal"
)

type Dependencies struct {
	Adapters *adapter.Registry
	Runtime  terminal.PseudoTerminalRuntime
	Store    store.Store
}
