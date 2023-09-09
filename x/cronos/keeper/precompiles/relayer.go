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
	"github.com/gogo/protobuf/proto"

	sdk "github.com/cosmos/cosmos-sdk/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	conntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibckeeper "github.com/cosmos/ibc-go/v7/modules/core/keeper"
)

type RelayerContract struct {
	BaseContract

	cdc       codec.Codec
	ibcKeeper *ibckeeper.Keeper
}

func NewRelayerContract(ibcKeeper *ibckeeper.Keeper, cdc codec.Codec) precompiles.StatefulPrecompiledContract {
	return &RelayerContract{
		BaseContract: NewBaseContract(
			common.BytesToAddress([]byte{101}),
		),
		ibcKeeper: ibcKeeper,
		cdc:       cdc,
	}
}

func (bc *RelayerContract) Address() common.Address {
	return common.BytesToAddress([]byte{101})
}

// RequiredGas calculates the contract gas use
func (bc *RelayerContract) RequiredGas(input []byte) uint64 {
	// TODO estimate required gas
	return 0
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

type MsgType interface {
	proto.Message
	*clienttypes.MsgCreateClient | *clienttypes.MsgUpdateClient | *clienttypes.MsgUpgradeClient | *clienttypes.MsgSubmitMisbehaviour |
		*conntypes.MsgConnectionOpenInit | *conntypes.MsgConnectionOpenTry | *conntypes.MsgConnectionOpenAck | *conntypes.MsgConnectionOpenConfirm |
		*chantypes.MsgChannelOpenInit | *chantypes.MsgChannelOpenTry | *chantypes.MsgChannelOpenAck | *chantypes.MsgChannelOpenConfirm | *chantypes.MsgRecvPacket | *chantypes.MsgAcknowledgement | *chantypes.MsgTimeout | *chantypes.MsgTimeoutOnClose
}

func unmarshalAndExec[T codec.ProtoMarshaler, U any](
	bc *RelayerContract,
	stateDB precompiles.ExtStateDB,
	input []byte,
	msg T,
	action func(context.Context, T) (*U, error),
) (*U, error) {
	if err := bc.cdc.Unmarshal(input, msg); err != nil {
		return nil, fmt.Errorf("fail to Unmarshal %T", msg)
	}

	var res *U
	if err := stateDB.ExecuteNativeAction(func(ctx sdk.Context) error {
		var err error
		res, err = action(ctx, msg)
		return err
	}); err != nil {
		return nil, err
	}

	return res, nil
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
	var err error
	var res codec.ProtoMarshaler
	switch prefix {
	case prefixCreateClient:
		res, err = unmarshalAndExec(bc, stateDB, input, new(clienttypes.MsgCreateClient), bc.ibcKeeper.CreateClient)
	case prefixUpdateClient:
		res, err = unmarshalAndExec(bc, stateDB, input, new(clienttypes.MsgUpdateClient), bc.ibcKeeper.UpdateClient)
	case prefixUpgradeClient:
		res, err = unmarshalAndExec(bc, stateDB, input, new(clienttypes.MsgUpgradeClient), bc.ibcKeeper.UpgradeClient)
	case prefixSubmitMisbehaviour:
		res, err = unmarshalAndExec(bc, stateDB, input, new(clienttypes.MsgSubmitMisbehaviour), bc.ibcKeeper.SubmitMisbehaviour)
	case prefixConnectionOpenInit:
		res, err = unmarshalAndExec(bc, stateDB, input, new(conntypes.MsgConnectionOpenInit), bc.ibcKeeper.ConnectionOpenInit)
	case prefixConnectionOpenTry:
		res, err = unmarshalAndExec(bc, stateDB, input, new(conntypes.MsgConnectionOpenTry), bc.ibcKeeper.ConnectionOpenTry)
	case prefixConnectionOpenAck:
		res, err = unmarshalAndExec(bc, stateDB, input, new(conntypes.MsgConnectionOpenAck), bc.ibcKeeper.ConnectionOpenAck)
	case prefixConnectionOpenConfirm:
		res, err = unmarshalAndExec(bc, stateDB, input, new(conntypes.MsgConnectionOpenConfirm), bc.ibcKeeper.ConnectionOpenConfirm)
	case prefixChannelOpenInit:
		res, err = unmarshalAndExec(bc, stateDB, input, new(chantypes.MsgChannelOpenInit), bc.ibcKeeper.ChannelOpenInit)
	case prefixChannelOpenTry:
		res, err = unmarshalAndExec(bc, stateDB, input, new(chantypes.MsgChannelOpenTry), bc.ibcKeeper.ChannelOpenTry)
	case prefixChannelOpenAck:
		res, err = unmarshalAndExec(bc, stateDB, input, new(chantypes.MsgChannelOpenAck), bc.ibcKeeper.ChannelOpenAck)
	case prefixChannelOpenConfirm:
		res, err = unmarshalAndExec(bc, stateDB, input, new(chantypes.MsgChannelOpenConfirm), bc.ibcKeeper.ChannelOpenConfirm)
	case prefixRecvPacket:
		res, err = unmarshalAndExec(bc, stateDB, input, new(chantypes.MsgRecvPacket), bc.ibcKeeper.RecvPacket)
	case prefixAcknowledgement:
		res, err = unmarshalAndExec(bc, stateDB, input, new(chantypes.MsgAcknowledgement), bc.ibcKeeper.Acknowledgement)
	case prefixTimeout:
		res, err = unmarshalAndExec(bc, stateDB, input, new(chantypes.MsgTimeout), bc.ibcKeeper.Timeout)
	case prefixTimeoutOnClose:
		res, err = unmarshalAndExec(bc, stateDB, input, new(chantypes.MsgTimeoutOnClose), bc.ibcKeeper.TimeoutOnClose)
	default:
		return nil, errors.New("unknown method")
	}
	if err != nil {
		return nil, err
	}
	return bc.cdc.Marshal(res)
}
