package precompiles

import (
	"bytes"
	"errors"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/controller/types"
	ibcchannelkeeper "github.com/cosmos/ibc-go/v7/modules/core/04-channel/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/evmos/ethermint/x/evm/keeper/precompiles"
)

var (
	RegisterAccountMethod abi.Method
	QueryAccountMethod    abi.Method
	IcaContractAddress    = common.BytesToAddress([]byte{102})
)

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	stringType, _ := abi.NewType("string", "", nil)
	bytesType, _ := abi.NewType("bytes", "", nil)
	RegisterAccountMethod = abi.NewMethod(
		"registerAccount", "registerAccount", abi.Function, "", false, false, abi.Arguments{abi.Argument{
			Name: "connectionID",
			Type: stringType,
		}, abi.Argument{
			Name: "owner",
			Type: addressType,
		}},
		nil,
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
			Type: bytesType,
		}},
	)
}

type IcaContract struct {
	cdc                 codec.Codec
	channelKeeper       *ibcchannelkeeper.Keeper
	icaControllerKeeper *icacontrollerkeeper.Keeper
}

func NewIcaContract(
	cdc codec.Codec,
	channelKeeper *ibcchannelkeeper.Keeper,
	icaControllerKeeper *icacontrollerkeeper.Keeper,
) precompiles.StatefulPrecompiledContract {
	return &IcaContract{
		cdc,
		channelKeeper,
		icaControllerKeeper,
	}
}

func (ic *IcaContract) Address() common.Address {
	return IcaContractAddress
}

// RequiredGas calculates the contract gas use
func (ic *IcaContract) RequiredGas(input []byte) uint64 {
	return RelayerContractRequiredGas
}

func (ic *IcaContract) IsStateful() bool {
	return true
}

func (ic *IcaContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	// parse input
	methodID := contract.Input[:4]
	stateDB := evm.StateDB.(precompiles.ExtStateDB)
	var err error
	var res codec.ProtoMarshaler
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
		account := args[1].(common.Address)
		txSender := evm.Origin
		if !isSameAddress(account, contract.CallerAddress) && !isSameAddress(account, txSender) {
			return nil, errors.New("unauthorized account registration")
		}
		// FIXME: pass version
		err = stateDB.ExecuteNativeAction(func(ctx sdk.Context) error {
			res, err = ic.icaControllerKeeper.RegisterInterchainAccount(ctx, &icacontrollertypes.MsgRegisterInterchainAccount{
				Owner:        sdk.AccAddress(account.Bytes()).String(),
				ConnectionId: connectionID,
			})
			return err
		})
	case string(QueryAccountMethod.ID):
		args, err := QueryAccountMethod.Inputs.Unpack(contract.Input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		connectionID := args[0].(string)
		account := args[1].(common.Address)
		err = stateDB.ExecuteNativeAction(func(ctx sdk.Context) error {
			res, err = ic.icaControllerKeeper.InterchainAccount(ctx, &icacontrollertypes.QueryInterchainAccountRequest{
				Owner:        sdk.AccAddress(account.Bytes()).String(),
				ConnectionId: connectionID,
			})
			return err
		})
	default:
		return nil, errors.New("unknown method")
	}
	if err != nil {
		return nil, err
	}
	return ic.cdc.Marshal(res)
}

func isSameAddress(a common.Address, b common.Address) bool {
	return bytes.Equal(a.Bytes(), b.Bytes())
}
