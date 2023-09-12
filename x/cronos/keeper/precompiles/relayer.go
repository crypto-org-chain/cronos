package precompiles

import (
	"encoding/binary"
	"errors"

	"github.com/cosmos/cosmos-sdk/codec"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	ibckeeper "github.com/cosmos/ibc-go/v7/modules/core/keeper"
	generated "github.com/crypto-org-chain/cronos/v2/bindings/cosmos/precompile/relayer"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/evmos/ethermint/x/evm/keeper/precompiles"
)

// TODO adjust the gas cost
const RelayerContractRequiredGas = 10000

var RelayerContractAddress = common.BytesToAddress([]byte{101})

type RelayerContract struct {
	BaseContract

	cdc       codec.Codec
	ibcKeeper *ibckeeper.Keeper
}

func NewRelayerContract(ibcKeeper *ibckeeper.Keeper, cdc codec.Codec) precompiles.StatefulPrecompiledContract {
	return &RelayerContract{
		BaseContract: NewBaseContract(
			generated.RelayerModuleMetaData.ABI,
			RelayerContractAddress,
		),
		ibcKeeper: ibcKeeper,
		cdc:       cdc,
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
	stateDB := evm.StateDB.(precompiles.ExtStateDB)

	var (
		err error
		res []byte
	)
	precompiles := []Registrable{bc}
	// TODO: handle dynamic args in EventTypePacket
	skipType := transfertypes.EventTypePacket
	switch prefix {
	case prefixCreateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.CreateClient, precompiles, skipType)
	case prefixUpdateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.UpdateClient, precompiles, skipType)
	case prefixUpgradeClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.UpgradeClient, precompiles, skipType)
	case prefixSubmitMisbehaviour:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.SubmitMisbehaviour, precompiles, skipType)
	case prefixConnectionOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.ConnectionOpenInit, precompiles, skipType)
	case prefixConnectionOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.ConnectionOpenTry, precompiles, skipType)
	case prefixConnectionOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.ConnectionOpenAck, precompiles, skipType)
	case prefixConnectionOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.ConnectionOpenConfirm, precompiles, skipType)
	case prefixChannelOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.ChannelOpenInit, precompiles, skipType)
	case prefixChannelOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.ChannelOpenTry, precompiles, skipType)
	case prefixChannelOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.ChannelOpenAck, precompiles, skipType)
	case prefixChannelOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.ChannelOpenConfirm, precompiles, skipType)
	case prefixRecvPacket:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.RecvPacket, precompiles, skipType)
	case prefixAcknowledgement:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.Acknowledgement, precompiles, skipType)
	case prefixTimeout:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.Timeout, precompiles, skipType)
	case prefixTimeoutOnClose:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, bc.ibcKeeper.TimeoutOnClose, precompiles, skipType)
	default:
		return nil, errors.New("unknown method")
	}
	return res, err
}
