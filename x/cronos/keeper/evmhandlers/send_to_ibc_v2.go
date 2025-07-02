package evmhandler

import (
	"fmt"
	"math/big"

	cronoskeeper "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
			Name:    "channel_id",
			Type:    uint256Type,
			Indexed: true,
		}, abi.Argument{
			Name:    "recipient",
			Type:    stringType,
			Indexed: false,
		}, abi.Argument{
			Name:    "amount",
			Type:    uint256Type,
			Indexed: false,
		}, abi.Argument{
			Name:    "extraData",
			Type:    bytesType,
			Indexed: false,
		}},
	)
}

// SendToIbcV2Handler handles `__CronosSendToIbc` log
type SendToIbcV2Handler struct {
	*SendToIbcHandler
}

func NewSendToIbcV2Handler(bankKeeper types.BankKeeper, cronosKeeper cronoskeeper.Keeper) *SendToIbcV2Handler {
	return &SendToIbcV2Handler{
		SendToIbcHandler: NewSendToIbcHandler(bankKeeper, cronosKeeper),
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
	if len(topics) != 3 {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Info("log signature matches but wrong number of indexed events")
		for i, topic := range topics {
			h.cronosKeeper.Logger(ctx).Debug(fmt.Sprintf("topic index: %d value: %s", i, topic.TerminalString()))
		}
		return nil
	}

	unpacked, err := SendToIbcEventV2.Inputs.Unpack(data)
	if err != nil {
		// log and ignore
		h.cronosKeeper.Logger(ctx).Error("log signature matches but failed to decode", "error", err)
		return nil
	}

	// needs to crope the extra bytes in the topic by using BytesToAddress
	sender := common.BytesToAddress(topics[1].Bytes())
	channelId := new(big.Int).SetBytes(topics[2].Bytes())
	recipient := unpacked[0].(string)
	amount := unpacked[1].(*big.Int)
	// extraData := unpacked[2].([]byte)

	return h.handle(ctx, contract, sender, recipient, amount, channelId)
}
