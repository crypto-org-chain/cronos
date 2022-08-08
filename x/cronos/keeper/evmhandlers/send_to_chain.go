package evmhandler

import (
	"fmt"
	"math/big"

	gravitytypes "github.com/peggyjv/gravity-bridge/module/v2/x/gravity/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cronoskeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

var _ types.EvmLogHandler = SendToChainHandler{}

const (
	SendToChainEventName         = "__CronosSendToChain"
	SendToChainResponseEventName = "__CronosSendToChainResponse"
)

var (
	// SendToChainEvent represent the signature of
	// `event __CronosSendToChain(address recipient, uint256 amount, uint256 bridge_fee)`
	SendToChainEvent abi.Event

	// SendToChainResponseEvent represent the signature of
	// `event __CronosSendToChainResponse(uint256 id)`
	SendToChainResponseEvent abi.Event
)

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)

	SendToChainEvent = abi.NewEvent(
		SendToChainEventName,
		SendToChainEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "sender",
			Type:    addressType,
			Indexed: false,
		}, abi.Argument{
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
		}, abi.Argument{
			Name:    "chain_id",
			Type:    uint256Type,
			Indexed: false,
		}},
	)
	SendToChainResponseEvent = abi.NewEvent(
		SendToChainResponseEventName,
		SendToChainResponseEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "id",
			Type:    uint256Type,
			Indexed: false,
		}},
	)
}

// SendToChainHandler handles `__CronosSendToChain` log
type SendToChainHandler struct {
	gravitySrv   gravitytypes.MsgServer
	bankKeeper   types.BankKeeper
	cronosKeeper cronoskeeper.Keeper
}

func NewSendToChainHandler(gravitySrv gravitytypes.MsgServer, bankKeeper types.BankKeeper, cronosKeeper cronoskeeper.Keeper) *SendToChainHandler {
	return &SendToChainHandler{
		gravitySrv:   gravitySrv,
		bankKeeper:   bankKeeper,
		cronosKeeper: cronosKeeper,
	}
}

func (h SendToChainHandler) EventID() common.Hash {
	return SendToChainEvent.ID
}

// Handle `__CronosSendToChain` log only if gravity is activated.
func (h SendToChainHandler) Handle(
	ctx sdk.Context,
	contract common.Address,
	data []byte,
	addLogToReceipt func(contractAddress common.Address, logSig common.Hash, logData []byte),
) error {
	if h.gravitySrv == nil {
		return fmt.Errorf("native action %s is not implemented", SendToChainEventName)
	}

	unpacked, err := SendToChainEvent.Inputs.Unpack(data)
	if err != nil {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Info("log signature matches but failed to decode")
		return nil
	}

	denom, found := h.cronosKeeper.GetDenomByContract(ctx, contract)
	if !found {
		return fmt.Errorf("contract %s is not connected to native token", contract)
	}

	if !types.IsValidGravityDenom(denom) && !types.IsValidCronosDenom(denom) {
		return fmt.Errorf("the native token associated with the contract %s is neither a gravity voucher or a cronos token", contract)
	}

	contractCosmosAddr := sdk.AccAddress(contract.Bytes())
	senderCosmosAddr := sdk.AccAddress(unpacked[0].(common.Address).Bytes())
	ethRecipient := unpacked[1].(common.Address)
	amount := sdk.NewIntFromBigInt(unpacked[2].(*big.Int))
	bridgeFee := sdk.NewIntFromBigInt(unpacked[3].(*big.Int))
	chainID := sdk.NewIntFromBigInt(unpacked[4].(*big.Int))

	if !chainID.Equal(sdk.NewInt(1)) && !chainID.Equal(sdk.NewInt(3)) &&
		!chainID.Equal(sdk.NewInt(4)) && !chainID.Equal(sdk.NewInt(5)) {
		return fmt.Errorf("only ethereum network is not supported")
	}

	coins := sdk.NewCoins(sdk.NewCoin(denom, amount.Add(bridgeFee)))
	if types.IsSourceCoin(denom) {
		// it is a source token, we need to mint coins
		if err = h.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
			return err
		}
		// send the coin to the user
		if err = h.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, senderCosmosAddr.Bytes(), coins); err != nil {
			return err
		}
	} else {
		// send coins from contract address to user address so that he will be able to cancel later on
		if err = h.bankKeeper.SendCoins(ctx, contractCosmosAddr, senderCosmosAddr.Bytes(), coins); err != nil {
			return err
		}
	}
	// Initialize a gravity transfer
	msg := gravitytypes.MsgSendToEthereum{
		Sender:            senderCosmosAddr.String(),
		EthereumRecipient: ethRecipient.Hex(),
		Amount:            sdk.NewCoin(denom, amount),
		BridgeFee:         sdk.NewCoin(denom, bridgeFee),
	}
	resp, err := h.gravitySrv.SendToEthereum(sdk.WrapSDKContext(ctx), &msg)
	if err != nil {
		return err
	}

	logData, _ := SendToChainResponseEvent.Inputs.Pack(big.NewInt(int64(resp.Id)))
	addLogToReceipt(contract, SendToChainResponseEvent.ID, logData)
	return nil
}
