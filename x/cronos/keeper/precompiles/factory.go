package precompiles

import (
	"github.com/ethereum/go-ethereum/common"
)

type Registrable interface {
	RegistryKey() common.Address
}

type precompileLog struct {
	eventType string
}

func (l *precompileLog) RegistryKey() string {
	return l.eventType
}

type Factory struct {
	events map[string]*precompileLog
}

func NewFactory(precompiles []Registrable) *Factory {
	f := &Factory{
		events: make(map[string]*precompileLog),
	}
	return f
}
