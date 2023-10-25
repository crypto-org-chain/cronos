package precompiles

import (
	"encoding/binary"
	"errors"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cronosevents "github.com/crypto-org-chain/cronos/v2/x/cronos/events"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
)

var (
	relayerContractAddress     = common.BytesToAddress([]byte{101})
	relayerGasRequiredByMethod = map[int]uint64{}
)

func init() {
	relayerGasRequiredByMethod[prefixCreateClient] = 200000
	relayerGasRequiredByMethod[prefixUpdateClient] = 400000
	relayerGasRequiredByMethod[prefixUpgradeClient] = 400000
	relayerGasRequiredByMethod[prefixSubmitMisbehaviour] = 100000
	relayerGasRequiredByMethod[prefixConnectionOpenInit] = 100000
	relayerGasRequiredByMethod[prefixConnectionOpenTry] = 100000
	relayerGasRequiredByMethod[prefixConnectionOpenAck] = 100000
	relayerGasRequiredByMethod[prefixConnectionOpenConfirm] = 100000
	relayerGasRequiredByMethod[prefixChannelOpenInit] = 100000
	relayerGasRequiredByMethod[prefixChannelOpenTry] = 100000
	relayerGasRequiredByMethod[prefixChannelOpenAck] = 100000
	relayerGasRequiredByMethod[prefixChannelOpenConfirm] = 100000
	relayerGasRequiredByMethod[prefixRecvPacket] = 250000
	relayerGasRequiredByMethod[prefixAcknowledgement] = 250000
	relayerGasRequiredByMethod[prefixTimeout] = 100000
	relayerGasRequiredByMethod[prefixTimeoutOnClose] = 100000
}

type RelayerContract struct {
	BaseContract

	cdc         codec.Codec
	ibcKeeper   types.IbcKeeper
	kvGasConfig storetypes.GasConfig
}

func NewRelayerContract(ibcKeeper types.IbcKeeper, cdc codec.Codec, kvGasConfig storetypes.GasConfig) vm.PrecompiledContract {
	return &RelayerContract{
		BaseContract: NewBaseContract(relayerContractAddress),
		ibcKeeper:    ibcKeeper,
		cdc:          cdc,
		kvGasConfig:  kvGasConfig,
	}
}

func (bc *RelayerContract) Address() common.Address {
	return relayerContractAddress
}

// RequiredGas calculates the contract gas use
func (bc *RelayerContract) RequiredGas(input []byte) uint64 {
	// base cost to prevent large input size
	baseCost := uint64(len(input)) * bc.kvGasConfig.WriteCostPerByte
	if len(input) < prefixSize4Bytes {
		return 0
	}
	prefix := int(binary.LittleEndian.Uint32(input[:prefixSize4Bytes]))
	requiredGas, ok := relayerGasRequiredByMethod[prefix]
	if ok {
		return requiredGas + baseCost
	}
	return baseCost
}

func (bc *RelayerContract) IsStateful() bool {
	return true
}

// prefix bytes for the relayer msg type
const (
	prefixSize4Bytes = 4
	// Client
	prefixCreateClient = iota + 1
	prefixUpdateClient
	prefixUpgradeClient
	prefixSubmitMisbehaviour
	// Connection
	prefixConnectionOpenInit
	prefixConnectionOpenTry
	prefixConnectionOpenAck
	prefixConnectionOpenConfirm
	// Channel
	prefixChannelOpenInit
	prefixChannelOpenTry
	prefixChannelOpenAck
	prefixChannelOpenConfirm
	prefixChannelCloseInit
	prefixChannelCloseConfirm
	prefixRecvPacket
	prefixAcknowledgement
	prefixTimeout
	prefixTimeoutOnClose
)

func (bc *RelayerContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if readonly {
		return nil, errors.New("the method is not readonly")
	}
	// parse input
	if len(contract.Input) < int(prefixSize4Bytes) {
		return nil, errors.New("data too short to contain prefix")
	}
	prefix := int(binary.LittleEndian.Uint32(contract.Input[:prefixSize4Bytes]))
	input := contract.Input[prefixSize4Bytes:]
	stateDB := evm.StateDB.(ExtStateDB)

	var (
		err error
		res []byte
	)
	precompileAddr := bc.Address()
	converter := cronosevents.RelayerConvertEvent
	switch prefix {
	case prefixCreateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.CreateClient, converter)
	case prefixUpdateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.UpdateClient, converter)
	case prefixUpgradeClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.UpgradeClient, converter)
	case prefixSubmitMisbehaviour:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.SubmitMisbehaviour, converter)
	case prefixConnectionOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenInit, converter)
	case prefixConnectionOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenTry, converter)
	case prefixConnectionOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenAck, converter)
	case prefixConnectionOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenConfirm, converter)
	case prefixChannelOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenInit, converter)
	case prefixChannelOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenTry, converter)
	case prefixChannelOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenAck, converter)
	case prefixChannelOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenConfirm, converter)
	case prefixRecvPacket:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.RecvPacket, converter)
	case prefixAcknowledgement:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.Acknowledgement, converter)
	case prefixTimeout:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.Timeout, converter)
	case prefixTimeoutOnClose:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.TimeoutOnClose, converter)
	default:
		return nil, errors.New("unknown method")
	}
	return res, err
}
