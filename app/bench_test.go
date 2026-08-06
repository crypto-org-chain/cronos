package app

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"path/filepath"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	memiavlstore "github.com/crypto-org-chain/cronos-store/store"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	srvflags "github.com/evmos/ethermint/server/flags"
	"github.com/evmos/ethermint/tests"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"
	sdkmath "cosmossdk.io/math"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const BlockSTMPreEstimate = true

// MinimalOptionsMap is a stub implementing AppOptions which can get data from a map
type MinimalOptionsMap map[string]interface{}

func (m MinimalOptionsMap) Get(key string) interface{} {
	if v, ok := m[key]; ok {
		return v
	}
	return interface{}(nil)
}

// BenchmarkERC20Transfer benchmarks execution of standard erc20 token transfer transactions
func BenchmarkERC20Transfer(b *testing.B) {
	b.Helper()
	b.Run("memdb", func(b *testing.B) {
		db := dbm.NewMemDB()
		benchmarkERC20Transfer(b, db, 1, MinimalOptionsMap{
			flags.FlagHome: b.TempDir(),
		})
	})
	b.Run("leveldb", func(b *testing.B) {
		homePath := b.TempDir()
		db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, filepath.Join(homePath, "data"))
		require.NoError(b, err)
		benchmarkERC20Transfer(b, db, 1, MinimalOptionsMap{
			flags.FlagHome: homePath,
		})
	})
	b.Run("memiavl", func(b *testing.B) {
		benchmarkERC20Transfer(b, nil, 1, MinimalOptionsMap{
			flags.FlagHome:           b.TempDir(),
			memiavlstore.FlagMemIAVL: true,
		})
	})
	for _, workers := range []int{1, 8, 16, 32} {
		b.Run(fmt.Sprintf("memiavl-stm-%d", workers), func(b *testing.B) {
			benchmarkERC20Transfer(b, nil, 1, MinimalOptionsMap{
				flags.FlagHome:                  b.TempDir(),
				memiavlstore.FlagMemIAVL:        true,
				memiavlstore.FlagCacheSize:      0,
				srvflags.EVMBlockExecutor:       "block-stm",
				srvflags.EVMBlockSTMWorkers:     workers,
				srvflags.EVMBlockSTMPreEstimate: BlockSTMPreEstimate,
			})
		})
	}
}

// BenchmarkERC20TransferBatched holds the EVM message count per block fixed and
// varies how many of those messages share one Cosmos tx. block-stm's unit of
// parallelism is the Cosmos tx, not the EVM message, so batching trades task
// count for task size at constant total work - and makes every re-execution
// replay the whole batch instead of a single message. Pre-estimate is varied
// too because it drives how conflicts are resolved, and the benchmark devnet
// runs with it off.
func BenchmarkERC20TransferBatched(b *testing.B) {
	b.Helper()
	for _, preEstimate := range []bool{false, true} {
		for _, msgsPerTx := range []int{1, 10, 100} {
			name := fmt.Sprintf("pre-estimate-%t/msgs-per-tx-%d", preEstimate, msgsPerTx)
			b.Run(name, func(b *testing.B) {
				benchmarkERC20Transfer(b, nil, msgsPerTx, MinimalOptionsMap{
					flags.FlagHome:                  b.TempDir(),
					memiavlstore.FlagMemIAVL:        true,
					memiavlstore.FlagCacheSize:      0,
					srvflags.EVMBlockExecutor:       "block-stm",
					srvflags.EVMBlockSTMWorkers:     16,
					srvflags.EVMBlockSTMPreEstimate: preEstimate,
				})
			})
		}
	}
}

// buildBatchTx packs signed messages into one Cosmos tx. It mirrors
// MsgEthereumTx.BuildTx, which handles only the single-message case, and sums
// fee and gas across the batch.
func buildBatchTx(b *testing.B, app *App, msgs []*evmtypes.MsgEthereumTx) []byte {
	b.Helper()

	builder, ok := app.TxConfig().NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	require.True(b, ok)

	option, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
	require.NoError(b, err)
	builder.SetExtensionOptions(option)

	fee := new(big.Int)
	var gas uint64
	// Only From and Raw survive encoding; the rest of MsgEthereumTx is derived.
	stripped := make([]sdk.Msg, len(msgs))
	for i, msg := range msgs {
		fee.Add(fee, msg.GetFee())
		gas += msg.GetGas()
		stripped[i] = &evmtypes.MsgEthereumTx{From: msg.From, Raw: msg.Raw}
	}
	require.NoError(b, builder.SetMsgs(stripped...))

	fees := make(sdk.Coins, 0, 1)
	if feeAmt := sdkmath.NewIntFromBigInt(fee); feeAmt.Sign() > 0 {
		fees = append(fees, sdk.NewCoin(evmtypes.DefaultEVMDenom, feeAmt))
	}
	builder.SetFeeAmount(fees)
	builder.SetGasLimit(gas)

	bz, err := app.TxConfig().TxEncoder()(builder.GetTx())
	require.NoError(b, err)
	return bz
}

type TestAccount struct {
	Address common.Address
	Priv    cryptotypes.PrivKey
	Nonce   uint64
}

// pass `nil` to db to use memiavl
func benchmarkERC20Transfer(b *testing.B, db dbm.DB, msgsPerTx int, appOpts servertypes.AppOptions) {
	b.Helper()
	txsPerBlock := 5000
	accounts := 100
	gasPrice := big.NewInt(100000000000)
	bigZero := big.NewInt(0)

	app := New(log.NewNopLogger(), db, true, appOpts, baseapp.SetChainID(TestAppChainID))
	defer app.Close()

	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)

	testAccounts := make([]TestAccount, accounts)
	addresses := make(map[common.Address]struct{}, accounts)
	for i := 0; i < accounts; i++ {
		priv, err := ethsecp256k1.GenerateKey()
		require.NoError(b, err)
		address := common.BytesToAddress(priv.PubKey().Address().Bytes())
		testAccounts[i] = TestAccount{Address: address, Priv: priv}
		addresses[address] = struct{}{}
	}
	// make sure the addresses are unique
	require.Equal(b, accounts, len(addresses))

	signMsg := func(acc *TestAccount, msg *evmtypes.MsgEthereumTx) {
		msg.From = acc.Address.Bytes()
		require.NoError(b, msg.Sign(ethSigner, tests.NewSigner(acc.Priv)))
	}

	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(b, err)

	consAddress := sdk.ConsAddress(pubKey.Address())
	validator := tmtypes.NewValidator(pubKey, 1)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})

	var (
		balances []banktypes.Balance
		accs     []authtypes.GenesisAccount
	)
	for _, acc := range testAccounts {
		baseAcct := authtypes.NewBaseAccount(acc.Priv.PubKey().Address().Bytes(), acc.Priv.PubKey(), 0, 0)
		accs = append(accs, baseAcct)
		balances = append(balances, banktypes.Balance{
			Address: baseAcct.GetAddress().String(),
			Coins:   sdk.NewCoins(sdk.NewCoin(evmtypes.DefaultEVMDenom, sdkmath.NewIntWithDecimal(10000000, 18))),
		})
	}
	genesisState, err := simtestutil.GenesisStateWithValSet(
		app.AppCodec(),
		app.DefaultGenesis(),
		valSet,
		accs,
		balances...,
	)
	require.NoError(b, err)

	appState, err := json.MarshalIndent(genesisState, "", "  ")
	require.NoError(b, err)

	blockParams := cmtproto.BlockParams{
		MaxBytes: math.MaxInt64,
		MaxGas:   math.MaxInt64,
	}
	consensusParams := *DefaultConsensusParams
	consensusParams.Block = &blockParams
	_, err = app.InitChain(&abci.RequestInitChain{
		ChainId:         TestAppChainID,
		AppStateBytes:   appState,
		ConsensusParams: &consensusParams,
	})
	require.NoError(b, err)

	// deploy contract
	ctx := app.GetContextForFinalizeBlock(nil).WithBlockHeader(cmtproto.Header{
		ChainID:         TestAppChainID,
		Height:          1,
		ProposerAddress: consAddress,
	})

	var contractAddr common.Address
	amount := int64(100000000)

	{
		ctx, write := ctx.CacheContext()
		contractAddr, err = app.CronosKeeper.DeployModuleCRC21(ctx, "test")
		require.NoError(b, err)
		for _, acc := range testAccounts {
			_, err = app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "mint_by_cronos_module", acc.Address, big.NewInt(amount))
			require.NoError(b, err)
		}
		write()
	}

	// do a dummy FinalizeBlock just to flush finalize state
	_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 1})
	require.NoError(b, err)
	_, err = app.Commit()
	require.NoError(b, err)

	// check remaining balance
	ctx = app.GetContextForCheckTx(nil).WithBlockHeader(cmtproto.Header{ProposerAddress: consAddress})
	ret, err := app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "balanceOf", testAccounts[0].Address)
	require.NoError(b, err)
	require.Equal(b, uint64(amount), binary.BigEndian.Uint64(ret[32-8:]))

	// check the code is deployed
	codeRsp, err := app.EvmKeeper.Code(app.GetContextForCheckTx(nil), &evmtypes.QueryCodeRequest{
		Address: contractAddr.Hex(),
	})
	require.NoError(b, err)
	require.NotEmpty(b, codeRsp.Code)

	// prepare transactions
	require.Zero(b, txsPerBlock%msgsPerTx, "msgsPerTx must divide txsPerBlock evenly")
	batchesPerBlock := txsPerBlock / msgsPerTx
	var transferTxs [][]byte
	for i := 0; i < b.N; i++ {
		for j := 0; j < batchesPerBlock; j++ {
			idx := rand.Int() % len(testAccounts)
			acct := &testAccounts[idx]
			recipient := common.BigToAddress(big.NewInt(int64(idx)))

			// One sender per batch: messages sharing a Cosmos tx carry
			// consecutive nonces, which only holds within a single account.
			msgs := make([]*evmtypes.MsgEthereumTx, msgsPerTx)
			for k := 0; k < msgsPerTx; k++ {
				data, err := types.ModuleCRC21Contract.ABI.Pack("transfer", recipient, big.NewInt(1))
				require.NoError(b, err)

				msg := evmtypes.NewTx(
					TestEthChainID,
					acct.Nonce,    // nonce
					&contractAddr, // to
					big.NewInt(0), // value
					210000,        // gas limit
					nil,           // gas price
					gasPrice,      // gasFeeCap
					bigZero,       // gasTipCap
					data,          // data
					nil,           // access list
				)
				acct.Nonce++

				signMsg(acct, msg)
				msgs[k] = msg
			}

			transferTxs = append(transferTxs, buildBatchTx(b, app, msgs))
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rsp, err := app.FinalizeBlock(&abci.RequestFinalizeBlock{
			Txs:             transferTxs[i*batchesPerBlock : (i+1)*batchesPerBlock],
			Height:          int64(i) + 2,
			ProposerAddress: consAddress,
		})
		require.NoError(b, err)
		for _, txResult := range rsp.TxResults {
			require.Equal(b, abci.CodeTypeOK, txResult.Code, txResult.Log)
		}
		_, err = app.Commit()
		require.NoError(b, err)
	}

	// check remaining balance
	ctx = app.GetContextForCheckTx(nil).WithBlockHeader(cmtproto.Header{ProposerAddress: consAddress})
	ret, err = app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "balanceOf", testAccounts[0].Address)
	require.NoError(b, err)
	require.Equal(b, uint64(amount)-testAccounts[0].Nonce, binary.BigEndian.Uint64(ret[32-8:]))
}
