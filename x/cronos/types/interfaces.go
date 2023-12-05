package types

import (
	context "context"
	"math/big"
	time "time"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	tmbytes "github.com/cometbft/cometbft/libs/bytes"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	icatypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/types"
	"github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	icaauthtypes "github.com/crypto-org-chain/cronos/v2/x/icaauth/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/v2/x/gravity/types"

	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	connectiontypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
)

// BankKeeper defines the expected interface needed to retrieve account balances.
type BankKeeper interface {
	SpendableCoins(ctx sdk.Context, addr sdk.AccAddress) sdk.Coins
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	SendCoins(ctx sdk.Context, senderAddr sdk.AccAddress, recipientAddr sdk.AccAddress, amt sdk.Coins) error

	GetDenomMetaData(ctx sdk.Context, denom string) (banktypes.Metadata, bool)
	SetDenomMetaData(ctx sdk.Context, denomMetaData banktypes.Metadata)
}

// TransferKeeper defines the expected interface needed to transfer coin through IBC.
type TransferKeeper interface {
	Transfer(goCtx context.Context, msg *types.MsgTransfer) (*types.MsgTransferResponse, error)
	GetDenomTrace(ctx sdk.Context, denomTraceHash tmbytes.HexBytes) (types.DenomTrace, bool)
}

// AccountKeeper defines the expected account keeper interface
type AccountKeeper interface {
	GetModuleAccount(ctx sdk.Context, moduleName string) authtypes.ModuleAccountI

	GetAccount(ctx sdk.Context, addr sdk.AccAddress) authtypes.AccountI
	SetAccount(ctx sdk.Context, account authtypes.AccountI)
}

// GravityKeeper defines the expected gravity keeper interface
type GravityKeeper interface {
	ERC20ToDenomLookup(ctx sdk.Context, tokenContract common.Address) (bool, string)
	IterateUnbatchedSendToEthereums(ctx sdk.Context, cb func(*gravitytypes.SendToEthereum) bool)
	GetParams(ctx sdk.Context) (params gravitytypes.Params)
	SetParams(ctx sdk.Context, params gravitytypes.Params)
}

// EvmLogHandler defines the interface for evm log handler
type EvmLogHandler interface {
	// Return the id of the log signature it handles
	EventID() common.Hash
	// Process the log
	Handle(ctx sdk.Context, contract common.Address, topics []common.Hash, data []byte,
		addLogToReceipt func(contractAddress common.Address, logSig common.Hash, logData []byte)) error
}

// EvmKeeper defines the interface for evm keeper
type EvmKeeper interface {
	GetNonce(ctx sdk.Context, addr common.Address) uint64
	ApplyMessage(ctx sdk.Context, msg core.Message, tracer vm.EVMLogger, commit bool) (*evmtypes.MsgEthereumTxResponse, error)
	GetParams(ctx sdk.Context) evmtypes.Params

	// to replay the messages
	EthereumTx(goCtx context.Context, msg *evmtypes.MsgEthereumTx) (*evmtypes.MsgEthereumTxResponse, error)
	GetBaseFee(ctx sdk.Context, ethCfg *params.ChainConfig) *big.Int
	DeductTxCostsFromUserBalance(ctx sdk.Context, fees sdk.Coins, from common.Address) error
	ChainID() *big.Int
}

// Icaauthkeeper defines the interface for icaauth keeper
type Icaauthkeeper interface {
	RegisterAccount(goCtx context.Context, msg *icaauthtypes.MsgRegisterAccount) (*icaauthtypes.MsgRegisterAccountResponse, error)
	InterchainAccountAddress(goCtx context.Context, req *icaauthtypes.QueryInterchainAccountAddressRequest) (*icaauthtypes.QueryInterchainAccountAddressResponse, error)
	SubmitTxWithArgs(goCtx context.Context, owner, connectionId string, timeoutDuration time.Duration, packetData icatypes.InterchainAccountPacketData) (string, *icaauthtypes.MsgSubmitTxResponse, error)
}

// CronosKeeper defines the interface for cronos keeper
type CronosKeeper interface {
	GetParams(ctx sdk.Context) (params Params)
}

// IbcKeeper defines the interface for ibc keeper
type IbcKeeper interface {
	CreateClient(goCtx context.Context, msg *clienttypes.MsgCreateClient) (*clienttypes.MsgCreateClientResponse, error)
	UpdateClient(goCtx context.Context, msg *clienttypes.MsgUpdateClient) (*clienttypes.MsgUpdateClientResponse, error)
	UpgradeClient(goCtx context.Context, msg *clienttypes.MsgUpgradeClient) (*clienttypes.MsgUpgradeClientResponse, error)
	SubmitMisbehaviour(goCtx context.Context, msg *clienttypes.MsgSubmitMisbehaviour) (*clienttypes.MsgSubmitMisbehaviourResponse, error)
	ConnectionOpenInit(goCtx context.Context, msg *connectiontypes.MsgConnectionOpenInit) (*connectiontypes.MsgConnectionOpenInitResponse, error)
	ConnectionOpenTry(goCtx context.Context, msg *connectiontypes.MsgConnectionOpenTry) (*connectiontypes.MsgConnectionOpenTryResponse, error)
	ConnectionOpenAck(goCtx context.Context, msg *connectiontypes.MsgConnectionOpenAck) (*connectiontypes.MsgConnectionOpenAckResponse, error)
	ConnectionOpenConfirm(goCtx context.Context, msg *connectiontypes.MsgConnectionOpenConfirm) (*connectiontypes.MsgConnectionOpenConfirmResponse, error)
	ChannelOpenInit(goCtx context.Context, msg *channeltypes.MsgChannelOpenInit) (*channeltypes.MsgChannelOpenInitResponse, error)
	ChannelOpenTry(goCtx context.Context, msg *channeltypes.MsgChannelOpenTry) (*channeltypes.MsgChannelOpenTryResponse, error)
	ChannelOpenAck(goCtx context.Context, msg *channeltypes.MsgChannelOpenAck) (*channeltypes.MsgChannelOpenAckResponse, error)
	ChannelOpenConfirm(goCtx context.Context, msg *channeltypes.MsgChannelOpenConfirm) (*channeltypes.MsgChannelOpenConfirmResponse, error)
	ChannelCloseInit(goCtx context.Context, msg *channeltypes.MsgChannelCloseInit) (*channeltypes.MsgChannelCloseInitResponse, error)
	ChannelCloseConfirm(goCtx context.Context, msg *channeltypes.MsgChannelCloseConfirm) (*channeltypes.MsgChannelCloseConfirmResponse, error)
	RecvPacket(goCtx context.Context, msg *channeltypes.MsgRecvPacket) (*channeltypes.MsgRecvPacketResponse, error)
	Acknowledgement(goCtx context.Context, msg *channeltypes.MsgAcknowledgement) (*channeltypes.MsgAcknowledgementResponse, error)
	Timeout(goCtx context.Context, msg *channeltypes.MsgTimeout) (*channeltypes.MsgTimeoutResponse, error)
	TimeoutOnClose(goCtx context.Context, msg *channeltypes.MsgTimeoutOnClose) (*channeltypes.MsgTimeoutOnCloseResponse, error)
}
