package app

import (
	"encoding/json"
	"math"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/evmos/ethermint/tests"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"
	sdkmath "cosmossdk.io/math"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// admissionFixture is a mempool.type=app App with funded EVM accounts and a
// signer, set up for exercising the lock-free admission path (InsertTxHandler).
type admissionFixture struct {
	app         *App
	accounts    []TestAccount
	consAddress sdk.ConsAddress
	ethSigner   ethtypes.Signer
}

// signTransfer builds and signs a plain EVM value-transfer tx from acc (to a
// fixed EOA), incrementing acc.Nonce. The signature is valid; tamperFrom swaps
// the From field to a different address post-sign so VerifyEthSig must reject.
func (f *admissionFixture) signTransfer(tb testing.TB, acc *TestAccount, tamperFrom *common.Address) []byte {
	tb.Helper()
	tx := evmtypes.NewTx(
		TestEthChainID,
		acc.Nonce,
		&common.Address{0x1},     // to: arbitrary EOA
		big.NewInt(0),            // value
		21000,                    // gas limit
		nil,                      // gas price
		big.NewInt(100000000000), // gasFeeCap
		big.NewInt(0),            // gasTipCap
		nil,                      // data
		nil,                      // access list
	)
	acc.Nonce++

	msg := tx
	msg.From = acc.Address.Bytes()
	require.NoError(tb, msg.Sign(f.ethSigner, tests.NewSigner(acc.Priv)))
	if tamperFrom != nil {
		msg.From = tamperFrom.Bytes() // recovered sender != From → verify fails
	}
	built, err := msg.BuildTx(f.app.TxConfig().NewTxBuilder(), evmtypes.DefaultEVMDenom)
	require.NoError(tb, err)
	bz, err := f.app.TxConfig().TxEncoder()(built)
	require.NoError(tb, err)
	return bz
}

// setupAdmissionApp builds a mempool.type=app App funded with `accounts` EVM
// accounts and runs one empty block so checkState is populated.
func setupAdmissionApp(tb testing.TB, accounts int) *admissionFixture {
	tb.Helper()

	appOpts := MinimalOptionsMap{
		flags.FlagHome:                tb.TempDir(),
		"mempool.type":                "app",
		"mempool.max-txs":             100000,
		"cronos.tx-decode-cache-size": 200000,
	}
	app := New(log.NewNopLogger(), dbm.NewMemDB(), true, appOpts, baseapp.SetChainID(TestAppChainID))
	tb.Cleanup(func() { _ = app.Close() })

	testAccounts := make([]TestAccount, accounts)
	for i := range testAccounts {
		priv, err := ethsecp256k1.GenerateKey()
		require.NoError(tb, err)
		testAccounts[i] = TestAccount{
			Address: common.BytesToAddress(priv.PubKey().Address().Bytes()),
			Priv:    priv,
		}
	}

	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(tb, err)
	consAddress := sdk.ConsAddress(pubKey.Address())
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(pubKey, 1)})

	var balances []banktypes.Balance
	var accs []authtypes.GenesisAccount
	for _, acc := range testAccounts {
		base := authtypes.NewBaseAccount(acc.Priv.PubKey().Address().Bytes(), acc.Priv.PubKey(), 0, 0)
		accs = append(accs, base)
		balances = append(balances, banktypes.Balance{
			Address: base.GetAddress().String(),
			Coins:   sdk.NewCoins(sdk.NewCoin(evmtypes.DefaultEVMDenom, sdkmath.NewIntWithDecimal(10000000, 18))),
		})
	}
	genesisState, err := simtestutil.GenesisStateWithValSet(app.AppCodec(), app.DefaultGenesis(), valSet, accs, balances...)
	require.NoError(tb, err)
	appState, err := json.MarshalIndent(genesisState, "", " ")
	require.NoError(tb, err)

	consensusParams := *DefaultConsensusParams
	blockParams := cmtproto.BlockParams{MaxBytes: math.MaxInt64, MaxGas: math.MaxInt64}
	consensusParams.Block = &blockParams
	_, err = app.InitChain(&abci.RequestInitChain{
		ChainId:         TestAppChainID,
		AppStateBytes:   appState,
		ConsensusParams: &consensusParams,
	})
	require.NoError(tb, err)

	// Flush an empty block so checkState reflects committed genesis state.
	_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 1, ProposerAddress: consAddress})
	require.NoError(tb, err)
	_, err = app.Commit()
	require.NoError(tb, err)

	return &admissionFixture{
		app:         app,
		accounts:    testAccounts,
		consAddress: consAddress,
		ethSigner:   ethtypes.LatestSignerForChainID(TestEthChainID),
	}
}

// TestPreVerifyEVMSig covers the Phase-3 pre-verify hook: valid EVM sig passes,
// a tampered From is rejected, and non-EVM / undecodable input passes through
// (nil) to the locked RunTx.
func TestPreVerifyEVMSig(t *testing.T) {
	f := setupAdmissionApp(t, 2)
	pv := newEVMSigPreVerifier(f.app.ChainID(), f.app.txDecoder)

	good := f.signTransfer(t, &f.accounts[0], nil)
	require.NoError(t, pv(good), "valid EVM signature must pass pre-verify")

	other := f.accounts[1].Address
	bad := f.signTransfer(t, &f.accounts[0], &other)
	require.Error(t, pv(bad), "tampered From must fail pre-verify")

	require.NoError(t, pv([]byte("not-a-tx")), "undecodable bytes pass through to locked verify")
}

// TestInsertTxConcurrentAdmission drives many concurrent InsertTx calls
// (pre-verify lock-free, then RunTx under the admission mutex). Run with -race
// to prove the path is concurrency-safe: the signer is pure and the decode cache
// is mutex-guarded. FinalizeBlock isn't run concurrently here — see
// TestAdmissionVsFinalizeBlockRace for the separate pre-existing keeper race.
func TestInsertTxConcurrentAdmission(t *testing.T) {
	const goroutines = 16
	const perG = 32
	f := setupAdmissionApp(t, goroutines)

	// Pre-build each goroutine's txs (sequential nonces per account) so signing
	// cost stays out of the concurrent section.
	txs := make([][][]byte, goroutines)
	for g := range goroutines {
		txs[g] = make([][]byte, perG)
		for i := range perG {
			txs[g][i] = f.signTransfer(t, &f.accounts[g], nil)
		}
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range perG {
				if _, err := f.app.InsertTx(&abci.RequestInsertTx{Tx: txs[g][i]}); err != nil {
					t.Errorf("g%d i%d: InsertTx transport error: %v", g, i, err)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}

// BenchmarkAdmission measures admitted tx/s through InsertTx at the current
// single-mutex ceiling (Phase-2 baseline / GATE). Pre-verify runs lock-free;
// RunTx is serialized by the admission mutex.
func BenchmarkAdmission(b *testing.B) {
	const goroutines = 16
	f := setupAdmissionApp(b, goroutines)

	// Pre-sign b.N txs per goroutine (distinct accounts → independent nonces).
	txs := make([][][]byte, goroutines)
	for g := range goroutines {
		txs[g] = make([][]byte, b.N)
		for i := range b.N {
			txs[g][i] = f.signTransfer(b, &f.accounts[g], nil)
		}
	}

	var admitted atomic.Int64
	b.ResetTimer()
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			for i := range b.N {
				resp, err := f.app.InsertTx(&abci.RequestInsertTx{Tx: txs[g][i]})
				if err == nil && resp.Code == abci.CodeTypeOK {
					admitted.Add(1)
				}
			}
		}(g)
	}
	wg.Wait()
	b.StopTimer()

	secs := b.Elapsed().Seconds()
	if secs > 0 {
		b.ReportMetric(float64(admitted.Load())/secs, "admit-tx/s")
	}
}
