package evmhandler

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cronoskeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

var _ types.EvmLogHandler = SendToIbcV2Handler{}

// SendToIbcEventV2 represent the signature of
// `event __CronosSendToIbc(address indexed sender, string indexed recipient, string indexed channel_id, uint256 amount, bytes extraData)`
var SendToIbcEventV2 abi.Event

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	stringType, _ := abi.NewType("string", "", nil)
	bytesType, _ := abi.NewType("bytes", "", nil)

	SendToIbcEventV2 = abi.NewEvent(
		SendToIbcEventName,
		SendToIbcEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "sender",
			Type:    addressType,
			Indexed: true,
		}, abi.Argument{
			Name:    "recipient",
			Type:    stringType,
			Indexed: true,
		}, abi.Argument{
			Name:    "amount",
			Type:    uint256Type,
			Indexed: false,
		}, abi.Argument{
			Name:    "channel_id",
			Type:    stringType,
			Indexed: true,
		}, abi.Argument{
			Name:    "extraData",
			Type:    bytesType,
			Indexed: false,
		}},
	)
}

// SendToIbcV2Handler handles `__CronosSendToIbc` log
type SendToIbcV2Handler struct {
	bankKeeper   types.BankKeeper
	cronosKeeper cronoskeeper.Keeper
}

func NewSendToIbcV2Handler(bankKeeper types.BankKeeper, cronosKeeper cronoskeeper.Keeper) *SendToIbcV2Handler {
	return &SendToIbcV2Handler{
		bankKeeper:   bankKeeper,
		cronosKeeper: cronosKeeper,
	}
}

func (h SendToIbcV2Handler) EventID() common.Hash {
	return SendToIbcEventV2.ID
}

func (h SendToIbcV2Handler) Handle(
	ctx sdk.Context,
	contract common.Address,
	topics []common.Hash,
	data []byte,
	_ func(contractAddress common.Address, logSig common.Hash, logData []byte),
) error {
	if len(topics) != 4 {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Info("log signature matches but wrong number of indexed events")
		return nil
	}

	unpacked, err := SendToIbcEventV2.Inputs.Unpack(data)
	if err != nil {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Info("log signature matches but failed to decode")
		return nil
	}

	denom, found := h.cronosKeeper.GetDenomByContract(ctx, contract)
	if !found {
		return fmt.Errorf("contract %s is not connected to native token", contract)
	}

	if !types.IsValidIBCDenom(denom) && !types.IsValidCronosDenom(denom) {
		return fmt.Errorf("the native token associated with the contract %s is neither an ibc voucher or a cronos token", contract)
	}

	contractAddr := sdk.AccAddress(contract.Bytes())
	sender := sdk.AccAddress(topics[1].Bytes())
	recipient := string(topics[2].Bytes())
	amount := sdk.NewIntFromBigInt(unpacked[0].(*big.Int))
	// channelId := string(topics[3].Bytes())
	// extraData := unpacked[1].([]byte)
	coins := sdk.NewCoins(sdk.NewCoin(denom, amount))

	if types.IsSourceCoin(denom) {
		// it is a source token, we need to mint coins
		if err = h.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
			return err
		}
		// send the coin to the user
		if err = h.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sender, coins); err != nil {
			return err
		}
	} else {
		// First, transfer IBC coin to user so that he will be the refunded address if transfer fails
		if err = h.bankKeeper.SendCoins(ctx, contractAddr, sender, coins); err != nil {
			return err
		}
	}
	// Initiate IBC transfer from sender account
	if err = h.cronosKeeper.IbcTransferCoins(ctx, sender.String(), recipient, coins); err != nil {
		return err
	}
	return nil
}
