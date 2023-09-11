package precompiles

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type BaseContract interface {
	Registrable
}

type baseContract struct {
	abi     abi.ABI
	address common.Address
}

func MustUnmarshalJSON(bz string) abi.ABI {
	var ret abi.ABI
	if err := ret.UnmarshalJSON([]byte(bz)); err != nil {
		panic(err)
	}
	return ret
}

func NewBaseContract(abiStr string, address common.Address) BaseContract {
	return &baseContract{
		abi:     MustUnmarshalJSON(abiStr),
		address: address,
	}
}

func (c *baseContract) RegistryKey() common.Address {
	return c.address
}

func (c *baseContract) ABIMethods() map[string]abi.Method {
	return c.abi.Methods
}

func (c *baseContract) ABIEvents() map[string]abi.Event {
	return c.abi.Events
}

func (c *baseContract) CustomValueDecoders() ValueDecoders {
	return nil
}

func (c *baseContract) PrecompileMethods() Methods {
	return Methods{}
}
