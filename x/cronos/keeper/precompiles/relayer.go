package precompiles

import (
	"errors"
	"fmt"

	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	connectiontypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibckeeper "github.com/cosmos/ibc-go/v7/modules/core/keeper"
	cronosevents "github.com/crypto-org-chain/cronos/v2/x/cronos/events"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/precompile/relayer"
)

var (
	irelayerABI                abi.ABI
	relayerContractAddress     = common.BytesToAddress([]byte{101})
	relayerGasRequiredByMethod = map[[4]byte]uint64{}
)

const (
	CreateClient                         = "createClient"
	UpdateClient                         = "updateClient"
	UpgradeClient                        = "upgradeClient"
	SubmitMisbehaviour                   = "submitMisbehaviour"
	ConnectionOpenInit                   = "connectionOpenInit"
	ConnectionOpenTry                    = "connectionOpenTry"
	ConnectionOpenAck                    = "connectionOpenAck"
	ConnectionOpenConfirm                = "connectionOpenConfirm"
	ChannelOpenInit                      = "channelOpenInit"
	ChannelOpenTry                       = "channelOpenTry"
	ChannelOpenAck                       = "channelOpenAck"
	ChannelOpenConfirm                   = "channelOpenConfirm"
	RecvPacket                           = "recvPacket"
	Acknowledgement                      = "acknowledgement"
	Timeout                              = "timeout"
	TimeoutOnClose                       = "timeoutOnClose"
	UpdateClientAndConnectionOpenTry     = "updateClientAndConnectionOpenTry"
	UpdateClientAndConnectionOpenConfirm = "updateClientAndConnectionOpenConfirm"
	UpdateClientAndChannelOpenTry        = "updateClientAndChannelOpenTry"
	UpdateClientAndChannelOpenConfirm    = "updateClientAndChannelOpenConfirm"
)

func init() {
	if err := irelayerABI.UnmarshalJSON([]byte(relayer.RelayerFunctionsMetaData.ABI)); err != nil {
		panic(err)
	}
	for methodName := range irelayerABI.Methods {
		var methodID [4]byte
		copy(methodID[:], irelayerABI.Methods[methodName].ID[:4])
		switch methodName {
		case CreateClient:
			relayerGasRequiredByMethod[methodID] = 200000
		case RecvPacket, Acknowledgement:
			relayerGasRequiredByMethod[methodID] = 250000
		case UpdateClient, UpgradeClient:
			relayerGasRequiredByMethod[methodID] = 400000
		default:
			relayerGasRequiredByMethod[methodID] = 100000
		}
	}
}

type RelayerContract struct {
	BaseContract

	cdc         codec.Codec
	ibcKeeper   *ibckeeper.Keeper
	kvGasConfig storetypes.GasConfig
}

func NewRelayerContract(ibcKeeper *ibckeeper.Keeper, cdc codec.Codec, kvGasConfig storetypes.GasConfig) vm.PrecompiledContract {
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
	var methodID [4]byte
	copy(methodID[:], input[:4])
	requiredGas, ok := relayerGasRequiredByMethod[methodID]
	if ok {
		return requiredGas + baseCost
	}
	return baseCost
}

func (bc *RelayerContract) IsStateful() bool {
	return true
}

func (bc *RelayerContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if readonly {
		return nil, errors.New("the method is not readonly")
	}
	// parse input
	methodID := contract.Input[:4]
	method, err := irelayerABI.MethodById(methodID)
	if err != nil {
		return nil, err
	}
	stateDB := evm.StateDB.(ExtStateDB)

	var res []byte
	precompileAddr := bc.Address()
	args, err := method.Inputs.Unpack(contract.Input[4:])
	if err != nil {
		return nil, errors.New("fail to unpack input arguments")
	}
	input := args[0].([]byte)
	converter := cronosevents.RelayerConvertEvent
	e := &Executor{
		bc.cdc, stateDB, contract.CallerAddress, precompileAddr, input, converter,
	}
	switch method.Name {
	case CreateClient:
		res, err = exec(e, bc.ibcKeeper.CreateClient)
	case UpdateClient:
		res, err = exec(e, bc.ibcKeeper.UpdateClient)
	case UpgradeClient:
		res, err = exec(e, bc.ibcKeeper.UpgradeClient)
	case SubmitMisbehaviour:
		res, err = exec(e, bc.ibcKeeper.SubmitMisbehaviour)
	case ConnectionOpenInit:
		res, err = exec(e, bc.ibcKeeper.ConnectionOpenInit)
	case ConnectionOpenTry:
		res, err = exec(e, bc.ibcKeeper.ConnectionOpenTry)
	case ConnectionOpenAck:
		res, err = exec(e, bc.ibcKeeper.ConnectionOpenAck)
	case ConnectionOpenConfirm:
		res, err = exec(e, bc.ibcKeeper.ConnectionOpenConfirm)
	case ChannelOpenInit:
		res, err = exec(e, bc.ibcKeeper.ChannelOpenInit)
	case ChannelOpenTry:
		res, err = exec(e, bc.ibcKeeper.ChannelOpenTry)
	case ChannelOpenAck:
		res, err = exec(e, bc.ibcKeeper.ChannelOpenAck)
	case ChannelOpenConfirm:
		res, err = exec(e, bc.ibcKeeper.ChannelOpenConfirm)
	case RecvPacket:
		res, err = exec(e, bc.ibcKeeper.RecvPacket)
	case Acknowledgement:
		res, err = exec(e, bc.ibcKeeper.Acknowledgement)
	case Timeout:
		res, err = exec(e, bc.ibcKeeper.Timeout)
	case TimeoutOnClose:
		res, err = exec(e, bc.ibcKeeper.TimeoutOnClose)
	case UpdateClientAndConnectionOpenTry:
		var msg0 clienttypes.MsgUpdateClient
		if err := e.cdc.Unmarshal(input, &msg0); err != nil {
			return nil, fmt.Errorf("fail to Unmarshal %T %w", msg0, err)
		}
		input1 := args[1].([]byte)
		if err := e.stateDB.ExecuteNativeAction(e.contract, e.converter, func(ctx sdk.Context) error {
			if _, err := bc.ibcKeeper.UpdateClient(ctx, &msg0); err != nil {
				return fmt.Errorf("fail to UpdateClient %w", err)
			}
			var msg1 connectiontypes.MsgConnectionOpenTry
			if err := e.cdc.Unmarshal(input1, &msg1); err != nil {
				return fmt.Errorf("fail to Unmarshal %T %w", msg1, err)
			}
			if _, err = bc.ibcKeeper.ConnectionOpenTry(ctx, &msg1); err != nil {
				return fmt.Errorf("fail to ConnectionOpenTry %w", err)
			}
			return err
		}); err != nil {
			return nil, err
		}
	case UpdateClientAndConnectionOpenConfirm:
		var msg0 clienttypes.MsgUpdateClient
		if err := e.cdc.Unmarshal(input, &msg0); err != nil {
			return nil, fmt.Errorf("fail to Unmarshal %T %w", msg0, err)
		}
		input1 := args[1].([]byte)
		if err := e.stateDB.ExecuteNativeAction(e.contract, e.converter, func(ctx sdk.Context) error {
			if _, err := bc.ibcKeeper.UpdateClient(ctx, &msg0); err != nil {
				return fmt.Errorf("fail to UpdateClient %w", err)
			}
			var msg1 connectiontypes.MsgConnectionOpenConfirm
			if err := e.cdc.Unmarshal(input1, &msg1); err != nil {
				return fmt.Errorf("fail to Unmarshal %T %w", msg1, err)
			}
			if _, err := bc.ibcKeeper.ConnectionOpenConfirm(ctx, &msg1); err != nil {
				return fmt.Errorf("fail to ConnectionOpenConfirm %w", err)
			}
			return err
		}); err != nil {
			return nil, err
		}
	case UpdateClientAndChannelOpenTry:
		var msg0 clienttypes.MsgUpdateClient
		if err := e.cdc.Unmarshal(input, &msg0); err != nil {
			return nil, fmt.Errorf("fail to Unmarshal %T %w", msg0, err)
		}
		input1 := args[1].([]byte)
		if err := e.stateDB.ExecuteNativeAction(e.contract, e.converter, func(ctx sdk.Context) error {
			if _, err := bc.ibcKeeper.UpdateClient(ctx, &msg0); err != nil {
				return fmt.Errorf("fail to UpdateClient %w", err)
			}
			var msg1 channeltypes.MsgChannelOpenTry
			if err := e.cdc.Unmarshal(input1, &msg1); err != nil {
				return fmt.Errorf("fail to Unmarshal %T %w", msg1, err)
			}
			if _, err := bc.ibcKeeper.ChannelOpenTry(ctx, &msg1); err != nil {
				return fmt.Errorf("fail to ChannelOpenTry %w", err)
			}
			return err
		}); err != nil {
			return nil, err
		}
	case UpdateClientAndChannelOpenConfirm:
		var msg0 clienttypes.MsgUpdateClient
		if err := e.cdc.Unmarshal(input, &msg0); err != nil {
			return nil, fmt.Errorf("fail to Unmarshal %T %w", msg0, err)
		}
		input1 := args[1].([]byte)
		if err := e.stateDB.ExecuteNativeAction(e.contract, e.converter, func(ctx sdk.Context) error {
			if _, err := bc.ibcKeeper.UpdateClient(ctx, &msg0); err != nil {
				return fmt.Errorf("fail to UpdateClient %w", err)
			}
			var msg1 channeltypes.MsgChannelOpenConfirm
			if err := e.cdc.Unmarshal(input1, &msg1); err != nil {
				return fmt.Errorf("fail to Unmarshal %T %w", msg1, err)
			}
			if _, err := bc.ibcKeeper.ChannelOpenConfirm(ctx, &msg1); err != nil {
				return fmt.Errorf("fail to ChannelOpenConfirm %w", err)
			}
			return err
		}); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unknown method")
	}
	return res, err
}
