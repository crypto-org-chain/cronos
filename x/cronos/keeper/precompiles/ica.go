package precompiles

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/controller/types"
	icatypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/types"
	ibcchannelkeeper "github.com/cosmos/ibc-go/v7/modules/core/04-channel/keeper"
	cronosevents "github.com/crypto-org-chain/cronos/v2/x/cronos/events"
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
	bytesType, _ := abi.NewType("bytes", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	RegisterAccountMethod = abi.NewMethod(
		"registerAccount", "registerAccount", abi.Function, "", false, false, abi.Arguments{abi.Argument{
			Name: "connectionID",
			Type: stringType,
		}, abi.Argument{
			Name: "owner",
			Type: addressType,
		}, abi.Argument{
			Name: "version",
			Type: stringType,
		}},
		abi.Arguments{abi.Argument{
			Name: "res",
			Type: bytesType,
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
			Type: bytesType,
		}},
	)
	SubmitMsgsMethod = abi.NewMethod(
		"submitMsgs", "submitMsgs", abi.Function, "", false, false, abi.Arguments{abi.Argument{
			Name: "connectionID",
			Type: stringType,
		}, abi.Argument{
			Name: "owner",
			Type: addressType,
		}, abi.Argument{
			Name: "data",
			Type: stringType,
		}, abi.Argument{
			Name: "timeout",
			Type: uint256Type,
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
) vm.PrecompiledContract {
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
	converter := cronosevents.IcaConvertEvent
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
		version := args[2].(string)
		txSender := evm.Origin
		if !isSameAddress(account, contract.CallerAddress) && !isSameAddress(account, txSender) {
			return nil, errors.New("unauthorized account registration")
		}
		err = stateDB.ExecuteNativeAction(precompileAddr, converter, func(ctx sdk.Context) error {
			res, err = ic.icaControllerKeeper.RegisterInterchainAccount(ctx, &icacontrollertypes.MsgRegisterInterchainAccount{
				Owner:        sdk.AccAddress(account.Bytes()).String(),
				ConnectionId: connectionID,
				Version:      version,
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
		err = stateDB.ExecuteNativeAction(precompileAddr, nil, func(ctx sdk.Context) error {
			res, err = ic.icaControllerKeeper.InterchainAccount(ctx, &icacontrollertypes.QueryInterchainAccountRequest{
				Owner:        sdk.AccAddress(account.Bytes()).String(),
				ConnectionId: connectionID,
			})
			return err
		})
	case string(SubmitMsgsMethod.ID):
		if readonly {
			return nil, errors.New("the method is not readonly")
		}
		args, err := SubmitMsgsMethod.Inputs.Unpack(contract.Input[4:])
		if err != nil {
			return nil, errors.New("fail to unpack input arguments")
		}
		connectionID := args[0].(string)
		account := args[1].(common.Address)
		data := args[2].(string)
		timeout := args[3].(*big.Int)
		txSender := evm.Origin
		if !isSameAddress(account, contract.CallerAddress) && !isSameAddress(account, txSender) {
			return nil, errors.New("unauthorized send tx")
		}

		var icaMsgData icatypes.InterchainAccountPacketData
		err = ic.cdc.UnmarshalJSON([]byte(data), &icaMsgData)
		if err != nil {
			panic(err)
		}
		err = stateDB.ExecuteNativeAction(precompileAddr, converter, func(ctx sdk.Context) error {
			res, err = ic.icaControllerKeeper.SendTx(ctx, &icacontrollertypes.MsgSendTx{ //nolint:staticcheck
				Owner:           sdk.AccAddress(account.Bytes()).String(),
				ConnectionId:    connectionID,
				PacketData:      icaMsgData,
				RelativeTimeout: timeout.Uint64(),
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
