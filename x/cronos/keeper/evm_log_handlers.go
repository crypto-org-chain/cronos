package keeper

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/v2/x/gravity/types"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

var (
	_ types.EvmLogHandler = SendToAccountHandler{}
	_ types.EvmLogHandler = SendToChainHandler{}
	_ types.EvmLogHandler = CancelSendToChainHandler{}
	_ types.EvmLogHandler = SendToIbcHandler{}
	_ types.EvmLogHandler = SendCroToIbcHandler{}
)

const (
	SendToAccountEventName       = "__CronosSendToAccount"
	SendToChainEventName         = "__CronosSendToChain"
	SendToChainResponseEventName = "__CronosSendToChainResponse"
	CancelSendToChainEventName   = "__CronosCancelSendToChain"
	SendToIbcEventName           = "__CronosSendToIbc"
	SendCroToIbcEventName        = "__CronosSendCroToIbc"
)

var (
	// SendToAccountEvent represent the signature of
	// `event __CronosSendToAccount(address recipient, uint256 amount)`
	SendToAccountEvent abi.Event

	// SendToChainEvent represent the signature of
	// `event __CronosSendToChain(address recipient, uint256 amount, uint256 bridge_fee)`
	SendToChainEvent abi.Event

	// SendToChainResponseEvent represent the signature of
	// `event __CronosSendToChainResponse(uint256 id)`
	SendToChainResponseEvent abi.Event

	// CancelSendToChainEvent represent the signature of
	// `event __CronosCancelSendToChain(uint256 id)`
	CancelSendToChainEvent abi.Event

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

func (h SendToAccountHandler) Handle(
	ctx sdk.Context,
	contract common.Address,
	data []byte,
	_ func(contractAddress common.Address, logSig common.Hash, logData []byte)) error {
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

// SendToChainHandler handles `__CronosSendToChain` log
type SendToChainHandler struct {
	gravitySrv   gravitytypes.MsgServer
	bankKeeper   types.BankKeeper
	cronosKeeper Keeper
}

func NewSendToChainHandler(gravitySrv gravitytypes.MsgServer, bankKeeper types.BankKeeper, cronosKeeper Keeper) *SendToChainHandler {
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
	addLogToReceipt func(contractAddress common.Address, logSig common.Hash, logData []byte)) error {
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

	if !types.IsValidGravityDenom(denom) {
		return fmt.Errorf("the native token associated with the contract %s is not a gravity voucher", contract)
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
	// First, transfer the coin to user so that he will be able to cancel later on
	if err = h.bankKeeper.SendCoins(ctx, contractCosmosAddr, senderCosmosAddr.Bytes(), coins); err != nil {
		return err
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

// CancelSendToChainHandler handles `__CronosCancelSendToChain` log
type CancelSendToChainHandler struct {
	gravitySrv    gravitytypes.MsgServer
	cronosKeeper  Keeper
	gravityKeeper types.GravityKeeper
}

func NewCancelSendToChainHandler(
	gravitySrv gravitytypes.MsgServer,
	cronosKeeper Keeper,
	gravityKeeper types.GravityKeeper) *CancelSendToChainHandler {
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
	_ func(contractAddress common.Address, logSig common.Hash, logData []byte)) error {
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

func (h SendToIbcHandler) Handle(
	ctx sdk.Context,
	contract common.Address,
	data []byte,
	_ func(contractAddress common.Address, logSig common.Hash, logData []byte)) error {
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

func (h SendCroToIbcHandler) Handle(
	ctx sdk.Context,
	contract common.Address,
	data []byte,
	_ func(contractAddress common.Address, logSig common.Hash, logData []byte)) error {
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
