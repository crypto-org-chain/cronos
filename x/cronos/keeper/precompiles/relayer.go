package precompiles

import (
	"encoding/binary"
	"errors"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	ibckeeper "github.com/cosmos/ibc-go/v7/modules/core/keeper"
)

// TODO adjust the gas cost
const RelayerContractRequiredGas = 10000

var RelayerContractAddress = common.BytesToAddress([]byte{101})

type RelayerContract struct {
	BaseContract

	cdc       codec.Codec
	ibcKeeper *ibckeeper.Keeper
}

func NewRelayerContract(ibcKeeper *ibckeeper.Keeper, cdc codec.Codec) vm.PrecompiledContract {
	return &RelayerContract{
		BaseContract: NewBaseContract(RelayerContractAddress),
		ibcKeeper:    ibcKeeper,
		cdc:          cdc,
	}
}

func (bc *RelayerContract) Address() common.Address {
	return RelayerContractAddress
}

// RequiredGas calculates the contract gas use
func (bc *RelayerContract) RequiredGas(input []byte) uint64 {
	return RelayerContractRequiredGas
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
	switch prefix {
	case prefixCreateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.CreateClient)
	case prefixUpdateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.UpdateClient)
	case prefixUpgradeClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.UpgradeClient)
	case prefixSubmitMisbehaviour:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.SubmitMisbehaviour)
	case prefixConnectionOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenInit)
	case prefixConnectionOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenTry)
	case prefixConnectionOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenAck)
	case prefixConnectionOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ConnectionOpenConfirm)
	case prefixChannelOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenInit)
	case prefixChannelOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenTry)
	case prefixChannelOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenAck)
	case prefixChannelOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.ChannelOpenConfirm)
	case prefixRecvPacket:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.RecvPacket)
	case prefixAcknowledgement:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.Acknowledgement)
	case prefixTimeout:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.Timeout)
	case prefixTimeoutOnClose:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, bc.ibcKeeper.TimeoutOnClose)
	default:
		return nil, errors.New("unknown method")
	}
	return res, err
}
