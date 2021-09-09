package keeper

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/x/gravity/types"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

var (
	_ types.EvmLogHandler = NativeTransferHandler{}
	_ types.EvmLogHandler = EthereumTransferHandler{}
	_ types.EvmLogHandler = IbcTransferHandler{}
)

const (
	NativeTransferEventName   = "__CronosSendToAccount"
	EthereumTransferEventName = "__CronosSendToEthereum"
	IbcTransferEventName      = "__CronosSendToIbc"
)

var (
	// NativeTransferEvent represent the signature of
	// `event __CronosSendToAccount(address recipient, uint256 amount)`
	NativeTransferEvent abi.Event

	// EthereumTransferEvent represent the signature of
	// `event __CronosSendToEthereum(address recipient, uint256 amount, uint256 bridge_fee)`
	EthereumTransferEvent abi.Event

	// IbcTransferEvent represent the signature of
	// `event __CronosSendToIbc(address sender, string recipient, uint256 amount)`
	IbcTransferEvent abi.Event
)

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	stringType, _ := abi.NewType("string", "", nil)
	NativeTransferEvent = abi.NewEvent(
		NativeTransferEventName,
		NativeTransferEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "recipient",
			Type:    addressType,
			Indexed: false,
		}, abi.Argument{
			Name:    "amount",
			Type:    uint256Type,
			Indexed: false,
		}},
	)
	EthereumTransferEvent = abi.NewEvent(
		EthereumTransferEventName,
		EthereumTransferEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "recipient",
			Type:    addressType,
			Indexed: false,
		}, abi.Argument{
			Name:    "amount",
			Type:    uint256Type,
			Indexed: false,
		}, abi.Argument{
			Name:    "bridge_fee",
			Type:    uint256Type,
			Indexed: false,
		}},
	)
	IbcTransferEvent = abi.NewEvent(
		IbcTransferEventName,
		IbcTransferEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "recipient",
			Type:    stringType,
			Indexed: false,
		}, abi.Argument{
			Name:    "amount",
			Type:    uint256Type,
			Indexed: false,
		}},
	)
}

// NativeTransferHandler handles `__CronosSendToAccount` log
type NativeTransferHandler struct {
	bankKeeper   types.BankKeeper
	cronosKeeper Keeper
}

func NewNativeTransferHandler(bankKeeper types.BankKeeper, cronosKeeper Keeper) *NativeTransferHandler {
	return &NativeTransferHandler{
		bankKeeper:   bankKeeper,
		cronosKeeper: cronosKeeper,
	}
}

func (h NativeTransferHandler) EventID() common.Hash {
	return NativeTransferEvent.ID
}

func (h NativeTransferHandler) Handle(ctx sdk.Context, contract common.Address, data []byte) error {
	unpacked, err := NativeTransferEvent.Inputs.Unpack(data)
	if err != nil {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Error("log signature matches but failed to decode", "error", err)
		return nil
	}

	denom, found := h.cronosKeeper.GetDenomByContract(ctx, contract)
	if !found {
		return fmt.Errorf("contract %s is not connected to native token", contract)
	}

	contractAddr := sdk.AccAddress(contract.Bytes())
	recipient := sdk.AccAddress(unpacked[0].(common.Address).Bytes())
	coins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromBigInt(unpacked[1].(*big.Int))))
	err = h.bankKeeper.SendCoins(ctx, contractAddr, recipient, coins)
	if err != nil {
		return err
	}

	return nil
}

// EthereumTransferHandler handles `__CronosSendToEthereum` log
type EthereumTransferHandler struct {
	gravitySrv   gravitytypes.MsgServer
	cronosKeeper Keeper
}

func NewEthereumTransferHandler(gravitySrv gravitytypes.MsgServer, cronosKeeper Keeper) *EthereumTransferHandler {
	return &EthereumTransferHandler{
		gravitySrv:   gravitySrv,
		cronosKeeper: cronosKeeper,
	}
}

func (h EthereumTransferHandler) EventID() common.Hash {
	return EthereumTransferEvent.ID
}

func (h EthereumTransferHandler) Handle(ctx sdk.Context, contract common.Address, data []byte) error {
	unpacked, err := EthereumTransferEvent.Inputs.Unpack(data)
	if err != nil {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Info("log signature matches but failed to decode")
		return nil
	}

	denom, found := h.cronosKeeper.GetDenomByContract(ctx, contract)
	if !found && !types.IsValidGravityDenom(denom) {
		return fmt.Errorf("contract %s is not connected to native token", contract)
	}

	contractAddr := sdk.AccAddress(contract.Bytes())
	ethRecipient := unpacked[0].(common.Address)
	amount := sdk.NewIntFromBigInt(unpacked[1].(*big.Int))
	bridgeFee := sdk.NewIntFromBigInt(unpacked[2].(*big.Int))
	msg := gravitytypes.MsgSendToEthereum{
		Sender:            contractAddr.String(),
		EthereumRecipient: ethRecipient.Hex(),
		Amount:            sdk.NewCoin(denom, amount),
		BridgeFee:         sdk.NewCoin(denom, bridgeFee),
	}
	_, err = h.gravitySrv.SendToEthereum(sdk.WrapSDKContext(ctx), &msg)
	if err != nil {
		return err
	}
	return nil
}

// IbcTransferHandler handles `__CronosSendToIbc` log
type IbcTransferHandler struct {
	cronosKeeper Keeper
}

func NewIbcTransferHandler(cronosKeeper Keeper) *IbcTransferHandler {
	return &IbcTransferHandler{
		cronosKeeper: cronosKeeper,
	}
}

func (h IbcTransferHandler) EventID() common.Hash {
	return IbcTransferEvent.ID
}

func (h IbcTransferHandler) Handle(ctx sdk.Context, contract common.Address, data []byte) error {
	unpacked, err := IbcTransferEvent.Inputs.Unpack(data)
	if err != nil {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Info("log signature matches but failed to decode")
		return nil
	}

	denom, found := h.cronosKeeper.GetDenomByContract(ctx, contract)
	if !found && !types.IsValidIBCDenom(denom) {
		return fmt.Errorf("contract %s is not connected to native token", contract)
	}

	contractAddr := sdk.AccAddress(contract.Bytes())
	recipient := unpacked[0].(string)
	amount := sdk.NewIntFromBigInt(unpacked[1].(*big.Int))
	coin := sdk.NewCoin(denom, amount)
	err = h.cronosKeeper.IbcTransferCoins(ctx, contractAddr.String(), recipient, sdk.NewCoins(coin))
	if err != nil {
		return err
	}
	return nil
}
