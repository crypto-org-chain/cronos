package mempool_test

import (
	"encoding/json"
	"math"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	cronos "github.com/crypto-org-chain/cronos/app"
	mempool "github.com/crypto-org-chain/cronos/app/mempool"
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
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type testAccount struct {
	Address common.Address
	Priv    cryptotypes.PrivKey
	Nonce   uint64
}

type minimalOptionsMap map[string]interface{}

func (m minimalOptionsMap) Get(key string) interface{} {
	if v, ok := m[key]; ok {
		return v
	}
	return interface{}(nil)
}

// admissionFixture is a mempool.type=app App with funded EVM accounts and a
// signer, set up for exercising the lock-free admission path (InsertTxHandler).
type admissionFixture struct {
	app         *cronos.App
	accounts    []testAccount
	consAddress sdk.ConsAddress
	ethSigner   ethtypes.Signer
}

// signTransfer builds and signs a plain EVM value-transfer tx from acc (to a
// fixed EOA), incrementing acc.Nonce. The signature is valid; tamperFrom swaps
// the From field to a different address post-sign so VerifyEthSig must reject.
func (f *admissionFixture) signTransfer(tb testing.TB, acc *testAccount, tamperFrom *common.Address) []byte {
	tb.Helper()
	tx := evmtypes.NewTx(
		cronos.TestEthChainID,
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

	appOpts := minimalOptionsMap{
		flags.FlagHome:         tb.TempDir(),
		"mempool.type":         "app",
		"mempool.max-txs":      100000,
		"cronos.tx-cache-size": 200000,
	}
	app := cronos.New(log.NewNopLogger(), dbm.NewMemDB(), true, appOpts, baseapp.SetChainID(cronos.TestAppChainID))
	tb.Cleanup(func() { _ = app.Close() })

	testAccounts := make([]testAccount, accounts)
	for i := range testAccounts {
		priv, err := ethsecp256k1.GenerateKey()
		require.NoError(tb, err)
		testAccounts[i] = testAccount{
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

	consensusParams := *cronos.DefaultConsensusParams
	blockParams := cmtproto.BlockParams{MaxBytes: math.MaxInt64, MaxGas: math.MaxInt64}
	consensusParams.Block = &blockParams
	_, err = app.InitChain(&abci.RequestInitChain{
		ChainId:         cronos.TestAppChainID,
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
		ethSigner:   ethtypes.LatestSignerForChainID(cronos.TestEthChainID),
	}
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

// TestReapVsRecheckConcurrentRealTxs races the reap path (EncodeTx marshal +
// HashTx) against the recheck path (signers() + RunTx(ExecModeReCheck)) on the
// SAME pooled tx pointers, with real ethermint wrappers. reap_test.go races
// insert-vs-reap but with stubs; TestInsertTxConcurrentAdmission uses real
// wrappers but distinct pointers per goroutine. Run with -race.
func TestReapVsRecheckConcurrentRealTxs(t *testing.T) {
	const accounts = 64
	const reapIters = 400
	const reapers = 4
	f := setupAdmissionApp(t, accounts)

	// One nonce-0 tx per account. Admitting populates the pool with real wrappers;
	// the decode cache reuses one pointer per tx (pool key == encCache key).
	txBytes := make([][]byte, accounts)
	for g := range accounts {
		txBytes[g] = f.signTransfer(t, &f.accounts[g], nil)
		resp, err := f.app.InsertTx(&abci.RequestInsertTx{Tx: txBytes[g]})
		require.NoError(t, err)
		require.Equal(t, abci.CodeTypeOK, resp.Code, "tx %d not admitted", g)
	}
	require.Equal(t, accounts, f.app.Mempool().CountTx())

	// Admission bumped each sender's checkState nonce; an empty block resets it to
	// committed (nonce 0) so recheck of these nonce-0 txs passes instead of
	// failing stale.
	_, err := f.app.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 2, ProposerAddress: f.consAddress})
	require.NoError(t, err)
	_, err = f.app.Commit()
	require.NoError(t, err)
	require.Equal(t, accounts, f.app.Mempool().CountTx(), "empty block must not evict")

	// nil encCache forces EncodeTx through the real marshal path, not a cache hit.
	reap := mempool.NewReapTxsHandler(
		f.app.Mempool(), f.app.TxConfig().TxEncoder(), nil,
		time.Second, 0, log.NewNopLogger(),
	)

	var wg sync.WaitGroup
	wg.Add(reapers + 1)

	go func() {
		defer wg.Done()
		f.app.MempoolAdmitter().StageRecheckSenders(2, txBytes)
		f.app.MempoolAdmitter().RecheckTxs()
	}()
	for r := range reapers {
		go func(r int) {
			defer wg.Done()
			for range reapIters {
				if _, err := reap(&abci.RequestReapTxs{}); err != nil {
					t.Errorf("reaper %d: %v", r, err)
					return
				}
			}
		}(r)
	}
	wg.Wait()

	// Survival confirms recheck passed and the shared pointers stayed live.
	require.Equal(t, accounts, f.app.Mempool().CountTx(), "no tx should be evicted")
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
