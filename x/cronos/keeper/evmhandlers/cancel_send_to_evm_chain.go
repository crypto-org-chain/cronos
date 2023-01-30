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

var _ types.EvmLogHandler = CancelSendToEvmChainHandler{}

const CancelSendToEvmChainEventName = "__CronosCancelSendToEvmChain"

// CancelSendToEvmChainEvent represent the signature of
// `event __CronosCancelSendToEvmChain(uint256 id)`
var CancelSendToEvmChainEvent abi.Event

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)

	CancelSendToEvmChainEvent = abi.NewEvent(
		CancelSendToEvmChainEventName,
		CancelSendToEvmChainEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "sender",
			Type:    addressType,
			Indexed: true,
		}, abi.Argument{
			Name:    "id",
			Type:    uint256Type,
			Indexed: false,
		}},
	)
}

// CancelSendToEvmChainHandler handles `__CronosCancelSendToEvmChain` log
type CancelSendToEvmChainHandler struct {
	gravitySrv    gravitytypes.MsgServer
	cronosKeeper  cronoskeeper.Keeper
	gravityKeeper types.GravityKeeper
}

func NewCancelSendToEvmChainHandler(
	gravitySrv gravitytypes.MsgServer,
	cronosKeeper cronoskeeper.Keeper,
	gravityKeeper types.GravityKeeper,
) *CancelSendToEvmChainHandler {
	return &CancelSendToEvmChainHandler{
		gravitySrv:    gravitySrv,
		cronosKeeper:  cronosKeeper,
		gravityKeeper: gravityKeeper,
	}
}

func (h CancelSendToEvmChainHandler) EventID() common.Hash {
	return CancelSendToEvmChainEvent.ID
}

// Handle `__CronosCancelSendToChain` log only if gravity is activated.
func (h CancelSendToEvmChainHandler) Handle(
	ctx sdk.Context,
	contract common.Address,
	topics []common.Hash,
	data []byte,
	_ func(contractAddress common.Address, logSig common.Hash, logData []byte),
) error {
	if h.gravitySrv == nil {
		return fmt.Errorf("native action %s is not implemented", CancelSendToEvmChainEventName)
	}

	if len(topics) != 2 {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Info("log signature matches but wrong number of indexed events")
		for i, topic := range topics {
			h.cronosKeeper.Logger(ctx).Debug(fmt.Sprintf("topic index: %d value: %s", i, topic.TerminalString()))
		}
		return nil
	}

	unpacked, err := CancelSendToEvmChainEvent.Inputs.Unpack(data)
	if err != nil {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Error("log signature matches but failed to decode", "error", err)
		return nil
	}

	// needs to crope the extra bytes in the topic and cast to a cosmos address
	senderCosmosAddr := sdk.AccAddress(common.BytesToAddress(topics[1].Bytes()).Bytes())
	id := sdk.NewIntFromBigInt(unpacked[0].(*big.Int))

	// Need to retrieve the batch to get the amount to refund
	var unbatched []*gravitytypes.SendToEthereum
	h.gravityKeeper.IterateUnbatchedSendToEthereums(ctx, func(ste *gravitytypes.SendToEthereum) bool {
		unbatched = append(unbatched, ste)
		return false
	})

	var send *gravitytypes.SendToEthereum
	for _, ste := range unbatched {
		if ste.Id == id.Uint64() {
			send = ste
		}
	}
	if send == nil {
		return fmt.Errorf("id not found or the transaction is already included in a batch")
	}

	_, denom := h.gravityKeeper.ERC20ToDenomLookup(ctx, common.HexToAddress(send.Erc20Token.Contract))
	if !types.IsValidGravityDenom(denom) {
		return fmt.Errorf("the native token associated with the contract %s is not a gravity voucher", send.Erc20Token.Contract)
	}

	// check that the event is emitted from the contract address that manage this token
	crc20Address, found := h.cronosKeeper.GetContractByDenom(ctx, denom)
	if !found {
		return fmt.Errorf("the native token %s is not associated with any contract address on cronos", denom)
	}
	if crc20Address != contract {
		return fmt.Errorf("cannot cancel a transfer of the native token %s from the contract address %s", denom, contract)
	}

	msg := gravitytypes.MsgCancelSendToEthereum{
		Sender: senderCosmosAddr.String(),
		Id:     id.Uint64(),
	}
	_, err = h.gravitySrv.CancelSendToEthereum(sdk.WrapSDKContext(ctx), &msg)
	if err != nil {
		return err
	}
	refundAmount := sdk.NewCoins(sdk.NewCoin(denom, send.Erc20Token.Amount.Add(send.Erc20Fee.Amount)))
	// If cancel has no error, we need to convert back the native token to evm tokens
	err = h.cronosKeeper.ConvertVouchersToEvmCoins(ctx, senderCosmosAddr.String(), refundAmount)
	if err != nil {
		return err
	}
	return nil
}
