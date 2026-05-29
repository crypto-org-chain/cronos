package precompiles

import (
	"errors"
	"math/big"

	"github.com/crypto-org-chain/cronos/x/exchange/keeper"
	"github.com/crypto-org-chain/cronos/x/exchange/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/evmos/ethermint/x/evm/statedb"

	"cosmossdk.io/math"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ---------------------------------------------------------------------------
// ExtStateDB — same interface as x/cronos/keeper/precompiles/interface.go
// ---------------------------------------------------------------------------

// ExtStateDB defines extra methods of statedb to support stateful precompiled contracts
type ExtStateDB interface {
	vm.StateDB
	ExecuteNativeAction(contract common.Address, converter statedb.EventConverter, action func(ctx sdk.Context) error) error
	Context() sdk.Context
}

// ---------------------------------------------------------------------------
// Constants — Hybrid settlement-only methods
// ---------------------------------------------------------------------------

const (
	SettleBatchMethodName      = "settleBatch"
	DepositMethodName          = "deposit"
	WithdrawMethodName         = "withdraw"
	CancelOnChainMethodName    = "cancelOnChain"
	GetMarketsMethodName       = "getMarkets"
	GetEscrowBalanceMethodName = "getEscrowBalance"
	GetTradeHistoryMethodName  = "getTradeHistory"
)

var (
	exchangeABI                 abi.ABI
	exchangeContractAddress     = common.BytesToAddress([]byte{103}) // 0x67
	exchangeGasRequiredByMethod = map[[4]byte]uint64{}
	exchangeMethodNamesByID     = map[[4]byte]string{}
)

// ExchangeABIJSON defines the ABI for the hybrid settlement precompile.
// This is the settlement-only interface — no placeOrder/cancelOrder (those are off-chain).
const ExchangeABIJSON = `[
	{
		"inputs": [
			{"name": "marketIds", "type": "uint256[]"},
			{"name": "makerAddresses", "type": "address[]"},
			{"name": "takerAddresses", "type": "address[]"},
			{"name": "makerSides", "type": "uint8[]"},
			{"name": "fillPrices", "type": "uint256[]"},
			{"name": "fillQuantities", "type": "uint256[]"},
			{"name": "makerNonces", "type": "uint256[]"},
			{"name": "takerNonces", "type": "uint256[]"},
			{"name": "makerSignatures", "type": "bytes[]"},
			{"name": "takerSignatures", "type": "bytes[]"}
		],
		"name": "settleBatch",
		"outputs": [{"name": "tradeIds", "type": "uint256[]"}],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "denom", "type": "string"},
			{"name": "amount", "type": "uint256"}
		],
		"name": "deposit",
		"outputs": [{"name": "success", "type": "bool"}],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "denom", "type": "string"},
			{"name": "amount", "type": "uint256"}
		],
		"name": "withdraw",
		"outputs": [{"name": "success", "type": "bool"}],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "nonce", "type": "uint256"}
		],
		"name": "cancelOnChain",
		"outputs": [{"name": "success", "type": "bool"}],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "getMarkets",
		"outputs": [
			{"name": "marketIds", "type": "uint256[]"},
			{"name": "baseDenoms", "type": "string[]"},
			{"name": "quoteDenoms", "type": "string[]"},
			{"name": "enabled", "type": "bool[]"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "user", "type": "address"},
			{"name": "denom", "type": "string"}
		],
		"name": "getEscrowBalance",
		"outputs": [{"name": "balance", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "marketId", "type": "uint256"},
			{"name": "limit", "type": "uint32"}
		],
		"name": "getTradeHistory",
		"outputs": [
			{"name": "tradeIds", "type": "uint256[]"},
			{"name": "prices", "type": "uint256[]"},
			{"name": "quantities", "type": "uint256[]"},
			{"name": "blockHeights", "type": "uint256[]"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "operator", "type": "address"},
			{"indexed": false, "name": "batchSize", "type": "uint256"},
			{"indexed": false, "name": "tradeCount", "type": "uint256"}
		],
		"name": "BatchSettled",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "marketId", "type": "uint256"},
			{"indexed": true, "name": "tradeId", "type": "uint256"},
			{"indexed": false, "name": "price", "type": "uint256"},
			{"indexed": false, "name": "quantity", "type": "uint256"}
		],
		"name": "TradeExecuted",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "depositor", "type": "address"},
			{"indexed": false, "name": "denom", "type": "string"},
			{"indexed": false, "name": "amount", "type": "uint256"}
		],
		"name": "Deposited",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "withdrawer", "type": "address"},
			{"indexed": false, "name": "denom", "type": "string"},
			{"indexed": false, "name": "amount", "type": "uint256"}
		],
		"name": "Withdrawn",
		"type": "event"
	}
]`

func init() {
	if err := exchangeABI.UnmarshalJSON([]byte(ExchangeABIJSON)); err != nil {
		panic(err)
	}

	for methodName := range exchangeABI.Methods {
		var methodID [4]byte
		copy(methodID[:], exchangeABI.Methods[methodName].ID[:4])
		switch methodName {
		case SettleBatchMethodName:
			exchangeGasRequiredByMethod[methodID] = 8000 // per-trade ~8K gas (base cost)
		case DepositMethodName:
			exchangeGasRequiredByMethod[methodID] = 5000
		case WithdrawMethodName:
			exchangeGasRequiredByMethod[methodID] = 5000
		case CancelOnChainMethodName:
			exchangeGasRequiredByMethod[methodID] = 3000
		case GetMarketsMethodName:
			exchangeGasRequiredByMethod[methodID] = 5000
		case GetEscrowBalanceMethodName:
			exchangeGasRequiredByMethod[methodID] = 3000
		case GetTradeHistoryMethodName:
			exchangeGasRequiredByMethod[methodID] = 5000
		default:
			exchangeGasRequiredByMethod[methodID] = 0
		}
		exchangeMethodNamesByID[methodID] = methodName
	}
}

// ---------------------------------------------------------------------------
// ExchangeContract — the precompiled contract (hybrid settlement mode)
// ---------------------------------------------------------------------------

// ExchangeContract is the precompiled contract for the exchange module at address 0x67.
// In hybrid mode, it provides settlement, deposit/withdraw, and query methods.
type ExchangeContract struct {
	keeper      keeper.Keeper
	kvGasConfig storetypes.GasConfig
}

// NewExchangeContract creates a new exchange precompile instance.
func NewExchangeContract(keeper keeper.Keeper, kvGasConfig storetypes.GasConfig) vm.PrecompiledContract {
	return &ExchangeContract{
		keeper:      keeper,
		kvGasConfig: kvGasConfig,
	}
}

func (ec *ExchangeContract) Name() string {
	return "exchange"
}

func (ec *ExchangeContract) Address() common.Address {
	return exchangeContractAddress
}

// RequiredGas returns the gas required for the given input
func (ec *ExchangeContract) RequiredGas(input []byte) uint64 {
	baseCost := uint64(len(input)) * ec.kvGasConfig.WriteCostPerByte
	if len(input) < 4 {
		return baseCost
	}
	var methodID [4]byte
	copy(methodID[:], input[:4])
	requiredGas, ok := exchangeGasRequiredByMethod[methodID]
	if ok {
		return requiredGas + baseCost
	}
	return baseCost
}

// Run executes the precompiled contract
func (ec *ExchangeContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	if len(contract.Input) < 4 {
		return nil, errors.New("input too short")
	}

	methodID := contract.Input[:4]
	method, err := exchangeABI.MethodById(methodID)
	if err != nil {
		return nil, err
	}

	stateDB := evm.StateDB.(ExtStateDB)
	precompileAddr := ec.Address()
	caller := contract.Caller()

	switch method.Name {
	case SettleBatchMethodName:
		return ec.settleBatch(method, contract.Input[4:], stateDB, precompileAddr, caller, readonly)
	case DepositMethodName:
		return ec.deposit(method, contract.Input[4:], stateDB, precompileAddr, caller, readonly)
	case WithdrawMethodName:
		return ec.withdraw(method, contract.Input[4:], stateDB, precompileAddr, caller, readonly)
	case CancelOnChainMethodName:
		return ec.cancelOnChain(method, contract.Input[4:], stateDB, precompileAddr, caller, readonly)
	case GetMarketsMethodName:
		return ec.getMarkets(method, stateDB)
	case GetEscrowBalanceMethodName:
		return ec.getEscrowBalance(method, contract.Input[4:], stateDB)
	case GetTradeHistoryMethodName:
		return ec.getTradeHistory(method, contract.Input[4:], stateDB)
	default:
		return nil, errors.New("unknown method")
	}
}

// ---------------------------------------------------------------------------
// Method implementations
// ---------------------------------------------------------------------------

func (ec *ExchangeContract) settleBatch(
	method *abi.Method, input []byte,
	stateDB ExtStateDB, precompileAddr, caller common.Address,
	readonly bool,
) ([]byte, error) {
	if readonly {
		return nil, errors.New("the method is not readonly")
	}

	args, err := method.Inputs.Unpack(input)
	if err != nil {
		return nil, errors.New("fail to unpack input arguments")
	}

	marketIDs := args[0].([]*big.Int)
	makerAddresses := args[1].([]common.Address)
	takerAddresses := args[2].([]common.Address)
	makerSides := args[3].([]uint8)
	fillPrices := args[4].([]*big.Int)
	fillQuantities := args[5].([]*big.Int)
	makerNonces := args[6].([]*big.Int)
	takerNonces := args[7].([]*big.Int)
	makerSignatures := args[8].([][]byte)
	takerSignatures := args[9].([][]byte)

	n := len(marketIDs)
	if n == 0 || n != len(makerAddresses) || n != len(takerAddresses) ||
		n != len(makerSides) || n != len(fillPrices) || n != len(fillQuantities) ||
		n != len(makerNonces) || n != len(takerNonces) ||
		n != len(makerSignatures) || n != len(takerSignatures) {
		return nil, errors.New("mismatched array lengths")
	}

	// Build matches
	matches := make([]types.MatchedTrade, n)
	for i := 0; i < n; i++ {
		matches[i] = types.MatchedTrade{
			MarketID:       marketIDs[i].Uint64(),
			MakerAddress:   sdk.AccAddress(makerAddresses[i].Bytes()).String(),
			TakerAddress:   sdk.AccAddress(takerAddresses[i].Bytes()).String(),
			MakerSide:      types.OrderSide(makerSides[i]),
			FillPrice:      math.LegacyNewDecFromBigInt(fillPrices[i]),
			FillQuantity:   math.LegacyNewDecFromBigInt(fillQuantities[i]),
			MakerNonce:     makerNonces[i].Uint64(),
			TakerNonce:     takerNonces[i].Uint64(),
			MakerSignature: makerSignatures[i],
			TakerSignature: takerSignatures[i],
		}
	}

	operatorAddr := sdk.AccAddress(caller.Bytes())
	var tradeIDs []*big.Int

	err = stateDB.ExecuteNativeAction(precompileAddr, nil, func(ctx sdk.Context) error {
		resp, err := ec.keeper.SettleBatch(ctx, operatorAddr.String(), matches)
		if err != nil {
			return err
		}
		tradeIDs = make([]*big.Int, len(resp.TradeIDs))
		for i, id := range resp.TradeIDs {
			tradeIDs[i] = new(big.Int).SetUint64(id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(tradeIDs)
}

func (ec *ExchangeContract) deposit(
	method *abi.Method, input []byte,
	stateDB ExtStateDB, precompileAddr, caller common.Address,
	readonly bool,
) ([]byte, error) {
	if readonly {
		return nil, errors.New("the method is not readonly")
	}

	args, err := method.Inputs.Unpack(input)
	if err != nil {
		return nil, errors.New("fail to unpack input arguments")
	}

	denom := args[0].(string)
	amountBig := args[1].(*big.Int)
	amount := math.NewIntFromBigInt(amountBig)
	depositorAddr := sdk.AccAddress(caller.Bytes())

	err = stateDB.ExecuteNativeAction(precompileAddr, nil, func(ctx sdk.Context) error {
		return ec.keeper.Deposit(ctx, depositorAddr, denom, amount)
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (ec *ExchangeContract) withdraw(
	method *abi.Method, input []byte,
	stateDB ExtStateDB, precompileAddr, caller common.Address,
	readonly bool,
) ([]byte, error) {
	if readonly {
		return nil, errors.New("the method is not readonly")
	}

	args, err := method.Inputs.Unpack(input)
	if err != nil {
		return nil, errors.New("fail to unpack input arguments")
	}

	denom := args[0].(string)
	amountBig := args[1].(*big.Int)
	amount := math.NewIntFromBigInt(amountBig)
	withdrawerAddr := sdk.AccAddress(caller.Bytes())

	err = stateDB.ExecuteNativeAction(precompileAddr, nil, func(ctx sdk.Context) error {
		return ec.keeper.Withdraw(ctx, withdrawerAddr, denom, amount)
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (ec *ExchangeContract) cancelOnChain(
	method *abi.Method, input []byte,
	stateDB ExtStateDB, precompileAddr, caller common.Address,
	readonly bool,
) ([]byte, error) {
	if readonly {
		return nil, errors.New("the method is not readonly")
	}

	args, err := method.Inputs.Unpack(input)
	if err != nil {
		return nil, errors.New("fail to unpack input arguments")
	}

	nonce := args[0].(*big.Int).Uint64()
	makerAddr := sdk.AccAddress(caller.Bytes())

	err = stateDB.ExecuteNativeAction(precompileAddr, nil, func(ctx sdk.Context) error {
		msgServer := keeper.NewMsgServerImpl(ec.keeper)
		_, err := msgServer.CancelOnChain(ctx, &types.MsgCancelOnChain{
			Maker: makerAddr.String(),
			Nonce: nonce,
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func (ec *ExchangeContract) getMarkets(
	method *abi.Method,
	stateDB ExtStateDB,
) ([]byte, error) {
	ctx := stateDB.Context()
	markets := ec.keeper.GetAllMarkets(ctx)

	marketIds := make([]*big.Int, len(markets))
	baseDenoms := make([]string, len(markets))
	quoteDenoms := make([]string, len(markets))
	enabled := make([]bool, len(markets))

	for i, m := range markets {
		marketIds[i] = new(big.Int).SetUint64(m.ID)
		baseDenoms[i] = m.BaseDenom
		quoteDenoms[i] = m.QuoteDenom
		enabled[i] = m.Enabled
	}

	return method.Outputs.Pack(marketIds, baseDenoms, quoteDenoms, enabled)
}

func (ec *ExchangeContract) getEscrowBalance(
	method *abi.Method, input []byte,
	stateDB ExtStateDB,
) ([]byte, error) {
	args, err := method.Inputs.Unpack(input)
	if err != nil {
		return nil, errors.New("fail to unpack input arguments")
	}

	userAddr := args[0].(common.Address)
	denom := args[1].(string)

	ctx := stateDB.Context()
	balance := ec.keeper.GetEscrowBalance(ctx, sdk.AccAddress(userAddr.Bytes()), denom)

	return method.Outputs.Pack(balance.BigInt())
}

func (ec *ExchangeContract) getTradeHistory(
	method *abi.Method, input []byte,
	stateDB ExtStateDB,
) ([]byte, error) {
	args, err := method.Inputs.Unpack(input)
	if err != nil {
		return nil, errors.New("fail to unpack input arguments")
	}

	marketID := args[0].(*big.Int).Uint64()
	limit := int(args[1].(uint32))
	if limit == 0 || limit > 100 {
		limit = 100
	}

	ctx := stateDB.Context()
	trades := ec.keeper.GetTradesByMarket(ctx, marketID, limit)

	tradeIds := make([]*big.Int, len(trades))
	prices := make([]*big.Int, len(trades))
	quantities := make([]*big.Int, len(trades))
	blockHeights := make([]*big.Int, len(trades))

	for i, t := range trades {
		tradeIds[i] = new(big.Int).SetUint64(t.ID)
		prices[i] = t.Price.TruncateInt().BigInt()
		quantities[i] = t.Quantity.TruncateInt().BigInt()
		blockHeights[i] = new(big.Int).SetInt64(t.BlockHeight)
	}

	return method.Outputs.Pack(tradeIds, prices, quantities, blockHeights)
}
