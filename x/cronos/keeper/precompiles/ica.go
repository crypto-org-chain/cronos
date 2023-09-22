package precompiles

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	icatypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/types"
	cronosevents "github.com/crypto-org-chain/cronos/v2/x/cronos/events"
	cronoseventstypes "github.com/crypto-org-chain/cronos/v2/x/cronos/events/types"
	icaauthkeeper "github.com/crypto-org-chain/cronos/v2/x/icaauth/keeper"
	"github.com/crypto-org-chain/cronos/v2/x/icaauth/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// TODO: Replace this const with adjusted gas cost corresponding to input when executing precompile contract.
const ICAContractRequiredGas = 10000

var (
	RegisterAccountMethod abi.Method
	QueryAccountMethod    abi.Method
	SubmitMsgsMethod      abi.Method
	IcaContractAddress    = common.BytesToAddress([]byte{102})
)

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	stringType, _ := abi.NewType("string", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	uint64Type, _ := abi.NewType("uint64", "", nil)
	boolType, _ := abi.NewType("bool", "", nil)
	RegisterAccountMethod = abi.NewMethod(
		"registerAccount", "registerAccount", abi.Function, "", false, false, abi.Arguments{abi.Argument{
			Name: "connectionID",
			Type: stringType,
		}, abi.Argument{
			Name: "version",
			Type: stringType,
		}},
		abi.Arguments{abi.Argument{
			Name: "res",
			Type: boolType,
		}},
	)
	QueryAccountMethod = abi.NewMethod(
		"queryAccount", "queryAccount", abi.Function, "", false, false, abi.Arguments{abi.Argument{
			Name: "connectionID",
			Type: stringType,
		}, abi.Argument{
			Name: "owner",
			Type: addressType,
		}},
		abi.Arguments{abi.Argument{
			Name: "res",
			Type: stringType,
		}},
	)
	SubmitMsgsMethod = abi.NewMethod(
		"submitMsgs", "submitMsgs", abi.Function, "", false, false, abi.Arguments{abi.Argument{
			Name: "connectionID",
			Type: stringType,
		}, abi.Argument{
			Name: "data",
			Type: stringType,
		}, abi.Argument{
			Name: "timeout",
			Type: uint256Type,
		}},
		abi.Arguments{abi.Argument{
			Name: "res",
			Type: uint64Type,
		}},
	)
}

type IcaContract struct {
	BaseContract

	cdc           codec.Codec
	icaauthKeeper *icaauthkeeper.Keeper
}

func NewIcaContract(icaauthKeeper *icaauthkeeper.Keeper, cdc codec.Codec) vm.PrecompiledContract {
	return &IcaContract{
		BaseContract:  NewBaseContract(IcaContractAddress),
		cdc:           cdc,
		icaauthKeeper: icaauthKeeper,
	}
}

func (ic *IcaContract) Address() common.Address {
	return IcaContractAddress
}

// RequiredGas calculates the contract gas use
func (ic *IcaContract) RequiredGas(input []byte) uint64 {
	return ICAContractRequiredGas
}

func (ic *IcaContract) IsStateful() bool {
	return true
}

func (ic *IcaContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	// parse input
	methodID := contract.Input[:4]
	stateDB := evm.StateDB.(ExtStateDB)
	precompileAddr := ic.Address()
	caller := contract.CallerAddress
	owner := sdk.AccAddress(caller.Bytes()).String()
	converter := cronosevents.IcaConvertEvent
	var execErr error
	switch string(methodID) {
	case string(RegisterAccountMethod.ID):
		if readonly {
			return nil, errors.New("the method is not readonly")
		}
		args, err := RegisterAccountMethod.Inputs.Unpack(contract.Input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		connectionID := args[0].(string)
		version := args[1].(string)
		execErr = stateDB.ExecuteNativeAction(precompileAddr, converter, func(ctx sdk.Context) error {
			_, err := ic.icaauthKeeper.RegisterAccount(ctx, &types.MsgRegisterAccount{
				Owner:        owner,
				ConnectionId: connectionID,
				Version:      version,
			})
			return err
		})
		if execErr != nil {
			return nil, execErr
		}
		return RegisterAccountMethod.Outputs.Pack(true)
	case string(QueryAccountMethod.ID):
		args, err := QueryAccountMethod.Inputs.Unpack(contract.Input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		connectionID := args[0].(string)
		account := args[1].(common.Address)
		owner := sdk.AccAddress(account.Bytes()).String()
		icaAddress := ""
		execErr = stateDB.ExecuteNativeAction(precompileAddr, nil, func(ctx sdk.Context) error {
			response, err := ic.icaauthKeeper.InterchainAccountAddress(ctx, &types.QueryInterchainAccountAddressRequest{
				Owner:        owner,
				ConnectionId: connectionID,
			})
			if err == nil && response != nil {
				icaAddress = response.InterchainAccountAddress
			}
			return err
		})
		if execErr != nil {
			return nil, execErr
		}
		return QueryAccountMethod.Outputs.Pack(icaAddress)
	case string(SubmitMsgsMethod.ID):
		if readonly {
			return nil, errors.New("the method is not readonly")
		}
		args, err := SubmitMsgsMethod.Inputs.Unpack(contract.Input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		connectionID := args[0].(string)
		data := args[1].(string)
		timeout := args[2].(*big.Int)
		var icaMsgData icatypes.InterchainAccountPacketData
		err = ic.cdc.UnmarshalJSON([]byte(data), &icaMsgData)
		if err != nil {
			return nil, errors.New("fail to unmarshal packet data")
		}
		timeoutDuration := time.Duration(timeout.Uint64())
		seq := uint64(0)
		execErr = stateDB.ExecuteNativeAction(precompileAddr, converter, func(ctx sdk.Context) error {
			response, err := ic.icaauthKeeper.SubmitTxWithArgs(
				ctx,
				owner,
				connectionID,
				timeoutDuration,
				icaMsgData,
			)
			if err == nil && response != nil {
				seq = response.Sequence
				ctx.EventManager().EmitEvents(sdk.Events{
					sdk.NewEvent(
						cronoseventstypes.EventTypeSubmitMsgsResult,
						sdk.NewAttribute(cronoseventstypes.AttributeKeySeq, fmt.Sprintf("%d", response.Sequence)),
					),
				})
			}
			return err
		})
		if execErr != nil {
			return nil, execErr
		}
		return SubmitMsgsMethod.Outputs.Pack(seq)
	default:
		return nil, errors.New("unknown method")
	}
}
