package orchestrator

import (
	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/store"
	"github.com/kamilandrzejrybacki-inc/clank/internal/terminal"
)

type Dependencies struct {
	Adapters *adapter.Registry
	Runtime  terminal.PseudoTerminalRuntime
	Store    store.Store
}
