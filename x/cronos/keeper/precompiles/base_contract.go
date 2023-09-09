package precompiles

import (
	"github.com/ethereum/go-ethereum/common"
)

type BaseContract interface {
	Registrable
}

type baseContract struct {
	address common.Address
}

func NewBaseContract(address common.Address) BaseContract {
	return &baseContract{
		address: address,
	}
}

func (c *baseContract) RegistryKey() common.Address {
	return c.address
}
