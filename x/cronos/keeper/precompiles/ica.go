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
	"github.com/crypto-org-chain/cronos/v2/x/cronos/events/bindings/cosmos/precompile/ica"
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
	icaABI             abi.ABI
	icaContractAddress = common.BytesToAddress([]byte{102})
)

func init() {
	if err := icaABI.UnmarshalJSON([]byte(ica.ICAModuleMetaData.ABI)); err != nil {
		panic(err)
	}
}

type IcaContract struct {
	BaseContract

	cdc           codec.Codec
	icaauthKeeper *icaauthkeeper.Keeper
}

func NewIcaContract(icaauthKeeper *icaauthkeeper.Keeper, cdc codec.Codec) vm.PrecompiledContract {
	return &IcaContract{
		BaseContract:  NewBaseContract(icaContractAddress),
		cdc:           cdc,
		icaauthKeeper: icaauthKeeper,
	}
}

func (ic *IcaContract) Address() common.Address {
	return icaContractAddress
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
	method, err := icaABI.MethodById(methodID)
	if err != nil {
		return nil, err
	}

	stateDB := evm.StateDB.(ExtStateDB)
	precompileAddr := ic.Address()
	caller := contract.CallerAddress
	owner := sdk.AccAddress(caller.Bytes()).String()
	converter := cronosevents.IcaConvertEvent
	var execErr error
	switch method.Name {
	case "registerAccount":
		if readonly {
			return nil, errors.New("the method is not readonly")
		}
		args, err := method.Inputs.Unpack(contract.Input[4:])
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
		return method.Outputs.Pack(true)
	case "queryAccount":
		args, err := method.Inputs.Unpack(contract.Input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		connectionID := args[0].(string)
		account := args[1].(common.Address)
		owner := sdk.AccAddress(account.Bytes()).String()
		icaAddress := ""
		response, err := ic.icaauthKeeper.InterchainAccountAddress(
			stateDB.CacheContext(),
			&types.QueryInterchainAccountAddressRequest{
				Owner:        owner,
				ConnectionId: connectionID,
			})
		if err != nil {
			return nil, err
		}
		if response != nil {
			icaAddress = response.InterchainAccountAddress
		}
		return method.Outputs.Pack(icaAddress)
	case "submitMsgs":
		if readonly {
			return nil, errors.New("the method is not readonly")
		}
		args, err := method.Inputs.Unpack(contract.Input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		connectionID := args[0].(string)
		data := args[1].([]byte)
		timeout := args[2].(*big.Int)
		icaMsgData := icatypes.InterchainAccountPacketData{
			Type: icatypes.EXECUTE_TX,
			Data: data,
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
		return method.Outputs.Pack(seq)
	default:
		return nil, errors.New("unknown method")
	}
}
