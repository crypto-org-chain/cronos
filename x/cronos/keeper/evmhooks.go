package keeper

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/x/gravity/types"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

var (
	// BankSendEvent represent the signature of
	// `event __CosmosNativeBankSend(address recipient, uint256 amount, string denom)`
	BankSendEvent abi.Event

	// GravitySendEvent represent the signature of
	// `event __CosmosNativeGravitySend(address recipient, uint256 amount, string denom)`
	GravitySendEvent abi.Event
)

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	stringType, _ := abi.NewType("string", "", nil)
	BankSendEvent = abi.NewEvent(
		"__CosmosNativeBankSend",
		"__CosmosNativeBankSend",
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
			Name:    "denom",
			Type:    stringType,
			Indexed: false,
		}},
	)
	GravitySendEvent = abi.NewEvent(
		"__CosmosNativeGravitySend",
		"__CosmosNativeGravitySend",
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
			Name:    "denom",
			Type:    stringType,
			Indexed: false,
		}},
	)
}

type BankSendHook struct {
	bankKeeper   types.BankKeeper
	cronosKeeper Keeper
}

func NewBankSendHook(bankKeeper types.BankKeeper, cronosKeeper Keeper) *BankSendHook {
	return &BankSendHook{
		bankKeeper:   bankKeeper,
		cronosKeeper: cronosKeeper,
	}
}

func (h BankSendHook) PostTxProcessing(ctx sdk.Context, txHash common.Hash, logs []*ethtypes.Log) error {
	for _, log := range logs {
		if len(log.Topics) == 0 || log.Topics[0] != BankSendEvent.ID {
			continue
		}
		unpacked, err := BankSendEvent.Inputs.Unpack(log.Data)
		if err != nil {
			ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
			continue
		}

		denom := unpacked[2].(string)
		// FIXME Verify denom and contract in mapping
		// after PR merged: https://github.com/crypto-org-chain/cronos/issues/24

		contract := sdk.AccAddress(log.Address.Bytes())
		recipient := sdk.AccAddress(unpacked[0].(common.Address).Bytes())
		coins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewIntFromBigInt(unpacked[1].(*big.Int))))
		err = h.bankKeeper.SendCoins(ctx, contract, recipient, coins)
		if err != nil {
			return err
		}
	}
	return nil
}

type GravitySendHook struct {
	gravitySrv   gravitytypes.MsgServer
	cronosKeeper Keeper
}

func NewGravitySendHook(gravitySrv gravitytypes.MsgServer, cronosKeeper Keeper) *GravitySendHook {
	return &GravitySendHook{
		gravitySrv:   gravitySrv,
		cronosKeeper: cronosKeeper,
	}
}

func (h GravitySendHook) PostTxProcessing(ctx sdk.Context, txHash common.Hash, logs []*ethtypes.Log) error {
	for _, log := range logs {
		if len(log.Topics) == 0 || log.Topics[0] != GravitySendEvent.ID {
			continue
		}
		unpacked, err := GravitySendEvent.Inputs.Unpack(log.Data)
		if err != nil {
			h.cronosKeeper.Logger(ctx).Info("log signature matches but failed to decode")
			continue
		}

		denom := unpacked[2].(string)
		// FIXME Verify denom and contract in mapping
		// after PR merged: https://github.com/crypto-org-chain/cronos/issues/24
		contract := sdk.AccAddress(log.Address.Bytes())
		recipient := sdk.AccAddress(unpacked[0].(common.Address).Bytes())
		coin := sdk.NewCoin(denom, sdk.NewIntFromBigInt(unpacked[1].(*big.Int)))
		msg := gravitytypes.MsgSendToEthereum{
			Sender:            contract.String(),
			EthereumRecipient: recipient.String(),
			Amount:            coin,
			// TODO bridge fee
			BridgeFee: sdk.NewCoin(denom, sdk.NewInt(0)),
		}
		_, err = h.gravitySrv.SendToEthereum(sdk.WrapSDKContext(ctx), &msg)
		if err != nil {
			return err
		}
	}
	return nil
}
