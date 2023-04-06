package evmhandler

import (
	"fmt"
	"math/big"

	gravitytypes "github.com/peggyjv/gravity-bridge/module/v2/x/gravity/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cronoskeeper "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
)

var _ types.EvmLogHandler = SendToEvmChainHandler{}

const (
	SendToEvmChainEventName         = "__CronosSendToEvmChain"
	SendToEvmChainResponseEventName = "__CronosSendToEvmChainResponse"
)

var (
	// SendToEvmChainEvent represent the signature of
	// `event __CronosSendToEvmChain(address indexed sender, address indexed recipient, uint256 indexed chain_id, uint256 amount, uint256 bridge_fee, bytes extraData)`
	SendToEvmChainEvent abi.Event

	// SendToEvmChainResponseEvent represent the signature of
	// `event __CronosSendToChainResponse(uint256 id)`
	SendToEvmChainResponseEvent abi.Event
)

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	bytesType, _ := abi.NewType("bytes", "", nil)

	SendToEvmChainEvent = abi.NewEvent(
		SendToEvmChainEventName,
		SendToEvmChainEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "sender",
			Type:    addressType,
			Indexed: true,
		}, abi.Argument{
			Name:    "recipient",
			Type:    addressType,
			Indexed: true,
		}, abi.Argument{
			Name:    "chain_id",
			Type:    uint256Type,
			Indexed: true,
		}, abi.Argument{
			Name:    "amount",
			Type:    uint256Type,
			Indexed: false,
		}, abi.Argument{
			Name:    "bridge_fee",
			Type:    uint256Type,
			Indexed: false,
		}, abi.Argument{
			Name:    "extraData",
			Type:    bytesType,
			Indexed: false,
		}},
	)
	SendToEvmChainResponseEvent = abi.NewEvent(
		SendToEvmChainResponseEventName,
		SendToEvmChainResponseEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "id",
			Type:    uint256Type,
			Indexed: false,
		}},
	)
}

// SendToEvmChainHandler handles `__CronosSendToEvmChain` log
type SendToEvmChainHandler struct {
	gravitySrv   gravitytypes.MsgServer
	bankKeeper   types.BankKeeper
	cronosKeeper cronoskeeper.Keeper
}

func NewSendToEvmChainHandler(gravitySrv gravitytypes.MsgServer, bankKeeper types.BankKeeper, cronosKeeper cronoskeeper.Keeper) *SendToEvmChainHandler {
	return &SendToEvmChainHandler{
		gravitySrv:   gravitySrv,
		bankKeeper:   bankKeeper,
		cronosKeeper: cronosKeeper,
	}
}

func (h SendToEvmChainHandler) EventID() common.Hash {
	return SendToEvmChainEvent.ID
}

// Handle `__CronosSendToChain` log only if gravity is activated.
func (h SendToEvmChainHandler) Handle(
	ctx sdk.Context,
	contract common.Address,
	topics []common.Hash,
	data []byte,
	addLogToReceipt func(contractAddress common.Address, logSig common.Hash, logData []byte),
) error {
	if h.gravitySrv == nil {
		return fmt.Errorf("native action %s is not implemented", SendToEvmChainEventName)
	}

	if len(topics) != 4 {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Info("log signature matches but wrong number of indexed events")
		for i, topic := range topics {
			h.cronosKeeper.Logger(ctx).Debug(fmt.Sprintf("topic index: %d value: %s", i, topic.TerminalString()))
		}
		return nil
	}

	unpacked, err := SendToEvmChainEvent.Inputs.Unpack(data)
	if err != nil {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Error("log signature matches but failed to decode", "error", err)
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
	// needs to crope the extra bytes in the topic and cast to a cosmos address
	senderCosmosAddr := sdk.AccAddress(common.BytesToAddress(topics[1].Bytes()).Bytes())
	ethRecipient := common.BytesToAddress(topics[2].Bytes())
	chainID := sdk.NewIntFromBigInt(new(big.Int).SetBytes(topics[3].Bytes()))
	amount := sdk.NewIntFromBigInt(unpacked[0].(*big.Int))
	bridgeFee := sdk.NewIntFromBigInt(unpacked[1].(*big.Int))

	if !chainID.Equal(sdk.NewInt(1)) && !chainID.Equal(sdk.NewInt(3)) &&
		!chainID.Equal(sdk.NewInt(4)) && !chainID.Equal(sdk.NewInt(5)) {
		return fmt.Errorf("only ethereum network is supported")
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

	logData, _ := SendToEvmChainResponseEvent.Inputs.Pack(big.NewInt(int64(resp.Id)))
	addLogToReceipt(contract, SendToEvmChainResponseEvent.ID, logData)
	return nil
}
