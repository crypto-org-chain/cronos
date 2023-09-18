package precompiles

import (
	"encoding/binary"
	"errors"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	ibckeeper "github.com/cosmos/ibc-go/v7/modules/core/keeper"
	cronosevents "github.com/crypto-org-chain/cronos/v2/x/cronos/events"
)

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

func (bc *RelayerContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if readonly {
		return nil, errors.New("the method is not readonly")
	}
	// parse input
	if len(contract.Input) < int(PrefixSize4Bytes) {
		return nil, errors.New("data too short to contain prefix")
	}
	prefix := int(binary.LittleEndian.Uint32(contract.Input[:PrefixSize4Bytes]))
	input := contract.Input[PrefixSize4Bytes:]
	stateDB := evm.StateDB.(ExtStateDB)

	var (
		err error
		res []byte
	)
	addr := bc.Address()
	caller := contract.CallerAddress
	converter := cronosevents.RelayerConvertEvent
	switch prefix {
	case prefixCreateClient:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.CreateClient, nil, converter)
	case prefixUpdateClient:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.UpdateClient, nil, converter)
	case prefixUpgradeClient:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.UpgradeClient, nil, converter)
	case prefixSubmitMisbehaviour:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.SubmitMisbehaviour, nil, converter)
	case prefixConnectionOpenInit:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.ConnectionOpenInit, nil, converter)
	case prefixConnectionOpenTry:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.ConnectionOpenTry, nil, converter)
	case prefixConnectionOpenAck:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.ConnectionOpenAck, nil, converter)
	case prefixConnectionOpenConfirm:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.ConnectionOpenConfirm, nil, converter)
	case prefixChannelOpenInit:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.ChannelOpenInit, nil, converter)
	case prefixChannelOpenTry:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.ChannelOpenTry, nil, converter)
	case prefixChannelOpenAck:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.ChannelOpenAck, nil, converter)
	case prefixChannelOpenConfirm:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.ChannelOpenConfirm, nil, converter)
	case prefixRecvPacket:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.RecvPacket, nil, converter)
	case prefixAcknowledgement:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.Acknowledgement, nil, converter)
	case prefixTimeout:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.Timeout, nil, converter)
	case prefixTimeoutOnClose:
		res, err = exec(bc.cdc, stateDB, caller, addr, input, bc.ibcKeeper.TimeoutOnClose, nil, converter)
	default:
		return nil, errors.New("unknown method")
	}
	return res, err
}
