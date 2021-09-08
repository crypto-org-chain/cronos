package keeper

import (
	"errors"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/x/gravity/types"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

var (
	// NativeTransferEvent represent the signature of
	// `event __CronosNativeTransfer(address recipient, uint256 amount, string denom)`
	NativeTransferEvent abi.Event

	// EthereumTransferEvent represent the signature of
	// `event __CronosEthereumTransfer(address recipient, uint256 amount, uint256 bridge_fee)`
	EthereumTransferEvent abi.Event
)

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	NativeTransferEvent = abi.NewEvent(
		"__CronosNativeTransfer",
		"__CronosNativeTransfer",
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
		"__CronosEthereumTransfer",
		"__CronosEthereumTransfer",
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
}

// NativeTransferHandler handles `__CronosNativeTransfer` log
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
		return errors.New("contract is not connected to native token")
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

// EthereumTransferHandler handles `__CosmosNativeGravitySend` log
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
	if !found {
		return errors.New("contract is not connected to native token")
	}

	contractAddr := sdk.AccAddress(contract.Bytes())
	recipient := sdk.AccAddress(unpacked[0].(common.Address).Bytes())
	amount := sdk.NewIntFromBigInt(unpacked[1].(*big.Int))
	bridgeFee := sdk.NewIntFromBigInt(unpacked[2].(*big.Int))
	msg := gravitytypes.MsgSendToEthereum{
		Sender:            contractAddr.String(),
		EthereumRecipient: recipient.String(),
		Amount:            sdk.NewCoin(denom, amount),
		// FIXME bridge fee?
		BridgeFee: sdk.NewCoin(denom, bridgeFee),
	}
	_, err = h.gravitySrv.SendToEthereum(sdk.WrapSDKContext(ctx), &msg)
	if err != nil {
		return err
	}
	return nil
}
