package precompiles

import (
	"github.com/cometbft/cometbft/libs/log"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/ethereum/go-ethereum/common"
)

type Registrable interface {
	RegistryKey() common.Address
}

type BaseContract interface {
	Registrable
	RequiredGas(input []byte) uint64
}

type baseContract struct {
	address                       common.Address
	kvGasConfig                   storetypes.GasConfig
	nameByMethod                  map[[4]byte]string
	gasByMethod                   map[[4]byte]uint64
	emptyGasIfInputLessThanPrefix bool
	logger                        log.Logger
}

func NewBaseContract(
	address common.Address,
	kvGasConfig storetypes.GasConfig,
	nameByMethod map[[4]byte]string,
	gasByMethod map[[4]byte]uint64,
	emptyGasIfInputLessThanPrefix bool,
	logger log.Logger,
) BaseContract {
	return &baseContract{
		address,
		kvGasConfig,
		nameByMethod,
		gasByMethod,
		emptyGasIfInputLessThanPrefix,
		logger,
	}
}

func (c *baseContract) RegistryKey() common.Address {
	return c.address
}

// RequiredGas calculates the contract gas use
func (c *baseContract) RequiredGas(input []byte) (gas uint64) {
	var methodID [4]byte
	copy(methodID[:], input[:4])
	inputLen := len(input)
	defer func() {
		method := c.nameByMethod[methodID]
		c.logger.Info("required", "gas", gas, "method", method, "len", inputLen)
	}()
	if c.emptyGasIfInputLessThanPrefix && inputLen < 4 {
		return
	}
	// base cost to prevent large input size
	gas = uint64(inputLen) * c.kvGasConfig.WriteCostPerByte
	if requiredGas, ok := c.gasByMethod[methodID]; ok {
		gas += requiredGas
	}
	return
}
