package types

import (
	context "context"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v3/modules/core/02-client/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/v2/x/gravity/types"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

// BankKeeper defines the expected interface needed to retrieve account balances.
type BankKeeper interface {
	GetBalance(ctx sdk.Context, addr sdk.AccAddress, denom string) sdk.Coin
	SendCoinsFromModuleToAccount(ctx sdk.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromAccountToModule(ctx sdk.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
	MintCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx sdk.Context, moduleName string, amt sdk.Coins) error
	SendCoins(ctx sdk.Context, senderAddr sdk.AccAddress, recipientAddr sdk.AccAddress, amt sdk.Coins) error
}

// TransferKeeper defines the expected interface needed to transfer coin through IBC.
type TransferKeeper interface {
	SendTransfer(
		ctx sdk.Context,
		sourcePort,
		sourceChannel string,
		token sdk.Coin,
		sender sdk.AccAddress,
		receiver string,
		timeoutHeight clienttypes.Height,
		timeoutTimestamp uint64,
	) error
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
}

// EvmLogHandler defines the interface for evm log handler
type EvmLogHandler interface {
	// Return the id of the log signature it handles
	EventID() common.Hash
	// Process the log
	Handle(ctx sdk.Context, contract common.Address, data []byte,
		addLogToReceipt func(contractAddress common.Address, logSig common.Hash, logData []byte)) error
}

// EvmKeeper defines the interface for evm keeper
type EvmKeeper interface {
	GetNonce(ctx sdk.Context, addr common.Address) uint64
	ApplyMessage(ctx sdk.Context, msg core.Message, tracer vm.EVMLogger, commit bool) (*evmtypes.MsgEthereumTxResponse, error)
	GetParams(ctx sdk.Context) evmtypes.Params

	// to replay the messages
	EthereumTx(goCtx context.Context, msg *evmtypes.MsgEthereumTx) (*evmtypes.MsgEthereumTxResponse, error)
	DeductTxCostsFromUserBalance(
		ctx sdk.Context, msgEthTx evmtypes.MsgEthereumTx, txData evmtypes.TxData, denom string, homestead, istanbul, london bool,
	) (sdk.Coins, error)
	ChainID() *big.Int
}
