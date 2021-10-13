package keeper

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

var (
	_ types.EvmLogHandler = SendToAccountHandler{}
	_ types.EvmLogHandler = SendToEthereumHandler{}
	_ types.EvmLogHandler = SendToIbcHandler{}
	_ types.EvmLogHandler = SendCroToIbcHandler{}
)

const (
	SendToAccountEventName  = "__CronosSendToAccount"
	SendToEthereumEventName = "__CronosSendToEthereum"
	SendToIbcEventName      = "__CronosSendToIbc"
	SendCroToIbcEventName   = "__CronosSendCroToIbc"
)

var (
	// SendToAccountEvent represent the signature of
	// `event __CronosSendToAccount(address recipient, uint256 amount)`
	SendToAccountEvent abi.Event

	// SendToEthereumEvent represent the signature of
	// `event __CronosSendToEthereum(address recipient, uint256 amount, uint256 bridge_fee)`
	SendToEthereumEvent abi.Event

	// SendToIbcEvent represent the signature of
	// `event __CronosSendToIbc(string recipient, uint256 amount)`
	SendToIbcEvent abi.Event

	// SendCroToIbcEvent represent the signature of
	// `event __CronosSendCroToIbc(string recipient, uint256 amount)`
	SendCroToIbcEvent abi.Event
)

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	stringType, _ := abi.NewType("string", "", nil)
	SendToAccountEvent = abi.NewEvent(
		SendToAccountEventName,
		SendToAccountEventName,
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
	SendToEthereumEvent = abi.NewEvent(
		SendToEthereumEventName,
		SendToEthereumEventName,
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
	SendToIbcEvent = abi.NewEvent(
		SendToIbcEventName,
		SendToIbcEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "sender",
			Type:    addressType,
			Indexed: false,
		}, abi.Argument{
			Name:    "recipient",
			Type:    stringType,
			Indexed: false,
		}, abi.Argument{
			Name:    "amount",
			Type:    uint256Type,
			Indexed: false,
		}},
	)
	SendCroToIbcEvent = abi.NewEvent(
		SendCroToIbcEventName,
		SendCroToIbcEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "sender",
			Type:    addressType,
			Indexed: false,
		}, abi.Argument{
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

// SendToAccountHandler handles `__CronosSendToAccount` log
type SendToAccountHandler struct {
	bankKeeper   types.BankKeeper
	cronosKeeper Keeper
}

func NewSendToAccountHandler(bankKeeper types.BankKeeper, cronosKeeper Keeper) *SendToAccountHandler {
	return &SendToAccountHandler{
		bankKeeper:   bankKeeper,
		cronosKeeper: cronosKeeper,
	}
}

func (h SendToAccountHandler) EventID() common.Hash {
	return SendToAccountEvent.ID
}

func (h SendToAccountHandler) Handle(ctx sdk.Context, contract common.Address, data []byte) error {
	unpacked, err := SendToAccountEvent.Inputs.Unpack(data)
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

// SendToEthereumHandler handles `__CronosSendToEthereum` log
type SendToEthereumHandler struct {
	cronosKeeper Keeper
}

func NewSendToEthereumHandler(cronosKeeper Keeper) *SendToEthereumHandler {
	return &SendToEthereumHandler{
		cronosKeeper: cronosKeeper,
	}
}

func (h SendToEthereumHandler) EventID() common.Hash {
	return SendToEthereumEvent.ID
}

func (h SendToEthereumHandler) Handle(ctx sdk.Context, contract common.Address, data []byte) error {
	return fmt.Errorf("native action %s is not implemented", SendToEthereumEventName)
}

// SendToIbcHandler handles `__CronosSendToIbc` log
type SendToIbcHandler struct {
	bankKeeper   types.BankKeeper
	cronosKeeper Keeper
}

func NewSendToIbcHandler(bankKeeper types.BankKeeper, cronosKeeper Keeper) *SendToIbcHandler {
	return &SendToIbcHandler{
		bankKeeper:   bankKeeper,
		cronosKeeper: cronosKeeper,
	}
}

func (h SendToIbcHandler) EventID() common.Hash {
	return SendToIbcEvent.ID
}

func (h SendToIbcHandler) Handle(ctx sdk.Context, contract common.Address, data []byte) error {
	unpacked, err := SendToIbcEvent.Inputs.Unpack(data)
	if err != nil {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Info("log signature matches but failed to decode")
		return nil
	}

	denom, found := h.cronosKeeper.GetDenomByContract(ctx, contract)
	if !found {
		return fmt.Errorf("contract %s is not connected to native token", contract)
	}

	if !types.IsValidIBCDenom(denom) {
		return fmt.Errorf("the native token associated with the contract %s is not an ibc voucher", contract)
	}

	contractAddr := sdk.AccAddress(contract.Bytes())
	sender := sdk.AccAddress(unpacked[0].(common.Address).Bytes())
	recipient := unpacked[1].(string)
	amount := sdk.NewIntFromBigInt(unpacked[2].(*big.Int))
	coins := sdk.NewCoins(sdk.NewCoin(denom, amount))

	// First, transfer IBC coin to user so that he will be the refunded address if transfer fails
	if err = h.bankKeeper.SendCoins(ctx, contractAddr, sender, coins); err != nil {
		return err
	}
	// Initiate IBC transfer from sender account
	if err = h.cronosKeeper.IbcTransferCoins(ctx, sender.String(), recipient, coins); err != nil {
		return err
	}
	return nil
}

// SendCroToIbcHandler handles `__CronosSendCroToIbc` log
type SendCroToIbcHandler struct {
	bankKeeper   types.BankKeeper
	cronosKeeper Keeper
}

func NewSendCroToIbcHandler(bankKeeper types.BankKeeper, cronosKeeper Keeper) *SendCroToIbcHandler {
	return &SendCroToIbcHandler{
		bankKeeper:   bankKeeper,
		cronosKeeper: cronosKeeper,
	}
}

func (h SendCroToIbcHandler) EventID() common.Hash {
	return SendCroToIbcEvent.ID
}

func (h SendCroToIbcHandler) Handle(ctx sdk.Context, contract common.Address, data []byte) error {
	unpacked, err := SendCroToIbcEvent.Inputs.Unpack(data)
	if err != nil {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Info("log signature matches but failed to decode")
		return nil
	}

	contractAddr := sdk.AccAddress(contract.Bytes())
	sender := sdk.AccAddress(unpacked[0].(common.Address).Bytes())
	recipient := unpacked[1].(string)
	amount := sdk.NewIntFromBigInt(unpacked[2].(*big.Int))
	evmDenom := h.cronosKeeper.GetEvmParams(ctx).EvmDenom
	coins := sdk.NewCoins(sdk.NewCoin(evmDenom, amount))
	// First, transfer IBC coin to user so that he will be the refunded address if transfer fails
	if err = h.bankKeeper.SendCoins(ctx, contractAddr, sender, coins); err != nil {
		return err
	}
	// Initiate IBC transfer from sender account
	if err = h.cronosKeeper.IbcTransferCoins(ctx, sender.String(), recipient, coins); err != nil {
		return err
	}
	return nil
}
