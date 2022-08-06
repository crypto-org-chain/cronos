package evmhandler

import (
	"fmt"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/v2/x/gravity/types"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cronoskeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

var _ types.EvmLogHandler = CancelSendToChainHandler{}

const CancelSendToChainEventName = "__CronosCancelSendToChain"

// CancelSendToChainEvent represent the signature of
// `event __CronosCancelSendToChain(uint256 id)`
var CancelSendToChainEvent abi.Event

func init() {
	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)

	CancelSendToChainEvent = abi.NewEvent(
		CancelSendToChainEventName,
		CancelSendToChainEventName,
		false,
		abi.Arguments{abi.Argument{
			Name:    "sender",
			Type:    addressType,
			Indexed: false,
		}, abi.Argument{
			Name:    "id",
			Type:    uint256Type,
			Indexed: false,
		}},
	)
}

// CancelSendToChainHandler handles `__CronosCancelSendToChain` log
type CancelSendToChainHandler struct {
	gravitySrv    gravitytypes.MsgServer
	cronosKeeper  cronoskeeper.Keeper
	gravityKeeper types.GravityKeeper
}

func NewCancelSendToChainHandler(
	gravitySrv gravitytypes.MsgServer,
	cronosKeeper cronoskeeper.Keeper,
	gravityKeeper types.GravityKeeper,
) *CancelSendToChainHandler {
	return &CancelSendToChainHandler{
		gravitySrv:    gravitySrv,
		cronosKeeper:  cronosKeeper,
		gravityKeeper: gravityKeeper,
	}
}

func (h CancelSendToChainHandler) EventID() common.Hash {
	return CancelSendToChainEvent.ID
}

// Handle `__CronosCancelSendToChain` log only if gravity is activated.
func (h CancelSendToChainHandler) Handle(
	ctx sdk.Context,
	_ common.Address,
	data []byte,
	_ func(contractAddress common.Address, logSig common.Hash, logData []byte),
) error {
	if h.gravitySrv == nil {
		return fmt.Errorf("native action %s is not implemented", CancelSendToChainEventName)
	}

	unpacked, err := CancelSendToChainEvent.Inputs.Unpack(data)
	if err != nil {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Info("log signature matches but failed to decode")
		return nil
	}

	senderCosmosAddr := sdk.AccAddress(unpacked[0].(common.Address).Bytes())
	id := sdk.NewIntFromBigInt(unpacked[1].(*big.Int))

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
