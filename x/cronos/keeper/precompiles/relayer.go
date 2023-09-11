package precompiles

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/evmos/ethermint/x/evm/keeper/precompiles"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
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

func NewRelayerContract(ibcKeeper *ibckeeper.Keeper, cdc codec.Codec) precompiles.StatefulPrecompiledContract {
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

type NativeMessage interface {
	codec.ProtoMarshaler
	GetSigners() []sdk.AccAddress
}

// exec is a generic function that executes the given action in statedb, and marshal/unmarshal the input/output
func exec[Req NativeMessage, Resp codec.ProtoMarshaler](
	cdc codec.Codec,
	stateDB precompiles.ExtStateDB,
	caller common.Address,
	input []byte,
	msg Req,
	action func(context.Context, Req) (Resp, error),
) ([]byte, error) {
	if err := cdc.Unmarshal(input, msg); err != nil {
		return nil, fmt.Errorf("fail to Unmarshal %T", msg)
	}

	signers := msg.GetSigners()
	if len(signers) != 1 {
		return nil, errors.New("don't support multi-signers message")
	}
	if common.BytesToAddress(signers[0].Bytes()) != caller {
		return nil, errors.New("caller is not authenticated")
	}

	var res Resp
	if err := stateDB.ExecuteNativeAction(func(ctx sdk.Context) error {
		var err error
		res, err = action(ctx, msg)
		return err
	}); err != nil {
		return nil, err
	}

	return cdc.Marshal(res)
}

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
	switch prefix {
	case prefixCreateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(clienttypes.MsgCreateClient), bc.ibcKeeper.CreateClient)
	case prefixUpdateClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(clienttypes.MsgUpdateClient), bc.ibcKeeper.UpdateClient)
	case prefixUpgradeClient:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(clienttypes.MsgUpgradeClient), bc.ibcKeeper.UpgradeClient)
	case prefixSubmitMisbehaviour:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(clienttypes.MsgSubmitMisbehaviour), bc.ibcKeeper.SubmitMisbehaviour)
	case prefixConnectionOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(conntypes.MsgConnectionOpenInit), bc.ibcKeeper.ConnectionOpenInit)
	case prefixConnectionOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(conntypes.MsgConnectionOpenTry), bc.ibcKeeper.ConnectionOpenTry)
	case prefixConnectionOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(conntypes.MsgConnectionOpenAck), bc.ibcKeeper.ConnectionOpenAck)
	case prefixConnectionOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(conntypes.MsgConnectionOpenConfirm), bc.ibcKeeper.ConnectionOpenConfirm)
	case prefixChannelOpenInit:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(chantypes.MsgChannelOpenInit), bc.ibcKeeper.ChannelOpenInit)
	case prefixChannelOpenTry:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(chantypes.MsgChannelOpenTry), bc.ibcKeeper.ChannelOpenTry)
	case prefixChannelOpenAck:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(chantypes.MsgChannelOpenAck), bc.ibcKeeper.ChannelOpenAck)
	case prefixChannelOpenConfirm:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(chantypes.MsgChannelOpenConfirm), bc.ibcKeeper.ChannelOpenConfirm)
	case prefixRecvPacket:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(chantypes.MsgRecvPacket), bc.ibcKeeper.RecvPacket)
	case prefixAcknowledgement:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(chantypes.MsgAcknowledgement), bc.ibcKeeper.Acknowledgement)
	case prefixTimeout:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(chantypes.MsgTimeout), bc.ibcKeeper.Timeout)
	case prefixTimeoutOnClose:
		res, err = exec(bc.cdc, stateDB, contract.CallerAddress, input, new(chantypes.MsgTimeoutOnClose), bc.ibcKeeper.TimeoutOnClose)
	default:
		return nil, errors.New("unknown method")
	}
	return res, err
}
