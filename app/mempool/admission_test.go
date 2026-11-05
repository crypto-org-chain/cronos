package mempool_test

import (
	"bytes"
	"encoding/json"
	"maps"
	"math"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"filippo.io/age"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/log/v2"
	sdkmath "cosmossdk.io/math"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/config"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	authcodec "github.com/cosmos/cosmos-sdk/x/auth/codec"
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
	return setupAdmissionAppWithOpts(tb, accounts, nil)
}

func setupAdmissionAppWithOpts(tb testing.TB, accounts int, overrides minimalOptionsMap) *admissionFixture {
	tb.Helper()

	appOpts := minimalOptionsMap{
		flags.FlagHome:         tb.TempDir(),
		"mempool.type":         "app",
		"mempool.max-txs":      100000,
		cronos.FlagTxCacheSize: 200000,
	}
	maps.Copy(appOpts, overrides)
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

// TestPreVerifyRejectsTamperedSig: tampered From rejected lock-free before RunTx.
func TestPreVerifyRejectsTamperedSig(t *testing.T) {
	f := setupAdmissionApp(t, 2)
	other := f.accounts[1].Address
	bad := f.signTransfer(t, &f.accounts[0], &other)
	resp, err := f.app.InsertTx(&abci.RequestInsertTx{Tx: bad})
	require.NoError(t, err)
	require.NotEqual(t, abci.CodeTypeOK, resp.Code, "tampered From must be rejected by pre-verifier")
}

// TestPreVerifyPassesThroughNonEVMTx: non-EVM tx skips pre-verifier, fails antehandler.
func TestPreVerifyPassesThroughNonEVMTx(t *testing.T) {
	f := setupAdmissionApp(t, 1)
	builder := f.app.TxConfig().NewTxBuilder()
	require.NoError(t, builder.SetMsgs(&banktypes.MsgSend{
		FromAddress: sdk.AccAddress(f.accounts[0].Address.Bytes()).String(),
		ToAddress:   sdk.AccAddress(f.accounts[0].Address.Bytes()).String(),
		Amount:      sdk.NewCoins(),
	}))
	bz, err := f.app.TxConfig().TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	resp, err := f.app.InsertTx(&abci.RequestInsertTx{Tx: bz})
	require.NoError(t, err)
	require.NotEqual(t, abci.CodeTypeOK, resp.Code, "unsigned tx must be rejected by antehandler")
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
		f.app.MempoolManager().StageRecheckSenders(2, txBytes)
		f.app.MempoolManager().RecheckTxs()
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

type SharedDecodedTxTestSuite struct {
	suite.Suite
}

func TestSharedDecodedTxTestSuite(t *testing.T) {
	suite.Run(t, new(SharedDecodedTxTestSuite))
}

var blockSTMOpts = minimalOptionsMap{
	server.FlagBlockExecutor:   config.BlockExecutorBlockSTM,
	server.FlagBlockSTMWorkers: 4,
}

// TestFirstUseOfSharedWrapper races first-time users of one decode-cached wrapper; run with -race.
func (s *SharedDecodedTxTestSuite) TestFirstUseOfSharedWrapper() {
	testCases := []struct {
		name      string
		overrides minimalOptionsMap
		race      func(t *testing.T, f *admissionFixture, bz []byte) []func()
	}{
		{
			name: "gossip admission vs ProcessProposal blocklist check",
			race: func(t *testing.T, f *admissionFixture, bz []byte) []func() {
				t.Helper()
				processProposal := newBlocklistProposalHandler(t, f).ProcessProposalHandler()
				return []func(){
					func() {
						resp, err := f.app.InsertTx(&abci.RequestInsertTx{Tx: bz})
						if assert.NoError(t, err) {
							assert.Equal(t, abci.CodeTypeOK, resp.Code)
						}
					},
					func() {
						resp, err := processProposal(sdk.Context{}, &abci.RequestProcessProposal{Txs: [][]byte{bz}})
						if assert.NoError(t, err) {
							assert.Equal(t, abci.ResponseProcessProposal_ACCEPT, resp.Status)
						}
					},
				}
			},
		},
		{
			name:      "duplicate tx bytes in one block-stm block",
			overrides: blockSTMOpts,
			race: func(t *testing.T, f *admissionFixture, bz []byte) []func() {
				t.Helper()
				return []func(){func() { finalizeTxs(t, f, bz, bz) }}
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			t := s.T()
			f := setupAdmissionAppWithOpts(t, 1, tc.overrides)
			bz := f.signTransfer(t, &f.accounts[0], nil)
			// Seed the decode cache so every racer gets the same wrapper.
			seeded, err := f.app.TxDecode(bz)
			require.NoError(t, err)
			again, err := f.app.TxDecode(bz)
			require.NoError(t, err)
			require.Same(t, seeded, again, "an uncached tx would make the race vacuous")

			fns := tc.race(t, f, bz)
			start := make(chan struct{})
			var wg sync.WaitGroup
			for _, fn := range fns {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					fn()
				}()
			}
			close(start)
			wg.Wait()
		})
	}
}

// Decode-time warm-up must not turn a tx error into a FinalizeBlock panic (chain halt).
func (s *SharedDecodedTxTestSuite) TestMalformedTxMatchesCacheOff() {
	cacheOff := minimalOptionsMap{cronos.FlagTxCacheSize: -1}
	testCases := []struct {
		name     string
		executor minimalOptionsMap
	}{
		{"sequential executor", nil},
		{"block-stm executor", blockSTMOpts},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			t := s.T()
			var results []*abci.ExecTxResult
			for _, cache := range []minimalOptionsMap{nil, cacheOff} {
				opts := maps.Clone(tc.executor)
				if opts == nil {
					opts = minimalOptionsMap{}
				}
				maps.Copy(opts, cache)
				f := setupAdmissionAppWithOpts(t, 1, opts)
				bz := nilFeeTx(t, f)
				var resp *abci.ResponseFinalizeBlock
				require.NotPanics(t, func() {
					var err error
					resp, err = f.app.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 2, ProposerAddress: f.consAddress, Txs: [][]byte{bz}})
					require.NoError(t, err)
				})
				results = append(results, resp.TxResults[0])
			}
			require.NotEqual(t, abci.CodeTypeOK, results[0].Code)
			require.Equal(t, results[1].Code, results[0].Code, results[0].Log)
			require.Equal(t, results[1].GasUsed, results[0].GasUsed)
		})
	}
}

// nilFeeTx is a decodable EVM transfer with empty auth_info, so AuthInfo.Fee is nil.
func nilFeeTx(t *testing.T, f *admissionFixture) []byte {
	t.Helper()
	var raw txtypes.TxRaw
	require.NoError(t, raw.Unmarshal(f.signTransfer(t, &f.accounts[0], nil)))
	raw.AuthInfoBytes = nil
	bz, err := raw.Marshal()
	require.NoError(t, err)
	return bz
}

func (s *SharedDecodedTxTestSuite) TestOnlyEVMTxsAreShared() {
	testCases := []struct {
		name       string
		build      func(t *testing.T, f *admissionFixture) []byte
		wantShared bool
	}{
		{
			name: "EVM transfer",
			build: func(t *testing.T, f *admissionFixture) []byte {
				t.Helper()
				return f.signTransfer(t, &f.accounts[0], nil)
			},
			wantShared: true,
		},
		{
			name: "cosmos MsgSend",
			build: func(t *testing.T, f *admissionFixture) []byte {
				t.Helper()
				from := sdk.AccAddress(f.accounts[0].Address.Bytes())
				b := f.app.TxConfig().NewTxBuilder()
				require.NoError(t, b.SetMsgs(banktypes.NewMsgSend(from, from, sdk.NewCoins(sdk.NewInt64Coin(evmtypes.DefaultEVMDenom, 1)))))
				b.SetGasLimit(100000)
				bz, err := f.app.TxConfig().TxEncoder()(b.GetTx())
				require.NoError(t, err)
				return bz
			},
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			t := s.T()
			f := setupAdmissionApp(t, 1)
			bz := tc.build(t, f)
			first, err := f.app.TxDecode(bz)
			require.NoError(t, err)
			second, err := f.app.TxDecode(bz)
			require.NoError(t, err)
			require.Equal(t, tc.wantShared, first == second)
		})
	}
}

// A non-empty blocklist makes ProcessProposal decode and check signers instead of fast-accepting.
func newBlocklistProposalHandler(t *testing.T, f *admissionFixture) *cronos.ProposalHandler {
	t.Helper()
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	addressCodec := authcodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	h := cronos.NewProposalHandler(f.app.TxDecode, identity, addressCodec)

	blocked, err := addressCodec.BytesToString(bytes.Repeat([]byte{0xb1}, 20))
	require.NoError(t, err)
	body, err := json.Marshal(cronos.BlockList{Addresses: []string{blocked}})
	require.NoError(t, err)
	var blob bytes.Buffer
	w, err := age.Encrypt(&blob, identity.Recipient())
	require.NoError(t, err)
	_, err = w.Write(body)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, h.SetBlockList(blob.Bytes()))
	return h
}

func finalizeTxs(t *testing.T, f *admissionFixture, txs ...[]byte) {
	t.Helper()
	resp, err := f.app.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 2, ProposerAddress: f.consAddress, Txs: txs})
	if assert.NoError(t, err) {
		assert.Equal(t, abci.CodeTypeOK, resp.TxResults[0].Code, resp.TxResults[0].Log)
	}
}

func TestAppInsertTx_AcceptsAndRejects(t *testing.T) {
	f := setupAdmissionApp(t, 2)

	valid := f.signTransfer(t, &f.accounts[0], nil)
	resp, err := f.app.MempoolClient().InsertTx(valid)
	require.NoError(t, err)
	require.Equal(t, abci.CodeTypeOK, resp.Code, "valid tx rejected: %s", resp.RawLog)
	require.Equal(t, 1, f.app.Mempool().CountTx(), "admitted tx must land in the pool")

	tampered := common.Address{0x9}
	// Tamper a fresh account so rejection is the forged From (sig mismatch), not
	// nonce/check-state mutated by the prior accepted tx.
	bz := f.signTransfer(t, &f.accounts[1], &tampered) // recovered sender != From
	resp, err = f.app.MempoolClient().InsertTx(bz)
	require.NoError(t, err, "admission failures map to a code, not a transport error")
	require.NotEqual(t, abci.CodeTypeOK, resp.Code, "tampered tx must be rejected")
	require.NotEmpty(t, resp.RawLog, "reject must carry a reason for the RPC caller")
}

func TestAppCommitTakesAdmissionGate(t *testing.T) {
	f := setupAdmissionApp(t, 1)
	_, err := f.app.FinalizeBlock(&abci.RequestFinalizeBlock{Height: 2, ProposerAddress: f.consAddress})
	require.NoError(t, err)

	// Overlapping Commit's own lock only costs it priority, which this test doesn't check.
	unlock := f.app.MempoolManager().LockForCommit()
	committed := make(chan error, 1)
	go func() {
		_, err := f.app.Commit()
		committed <- err
	}()

	select {
	case <-committed:
		unlock()
		t.Fatal("App.Commit ran while the admission gate was held")
	case <-time.After(50 * time.Millisecond):
	}
	// Catches a Commit that runs BaseApp.Commit first and only then blocks on the gate.
	require.Equal(t, int64(1), f.app.LastBlockHeight(), "BaseApp.Commit ran before the gate was released")
	unlock()
	select {
	case err := <-committed:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("App.Commit did not finish after the gate was released")
	}
	require.Equal(t, int64(2), f.app.LastBlockHeight())
}

func TestAppMempoolClientNilWithoutAppMempool(t *testing.T) {
	appMempool := setupAdmissionApp(t, 1).app
	require.NotNil(t, appMempool.MempoolManager())
	require.NotNil(t, appMempool.MempoolClient(), "app mempool exposes a client")

	floodOpts := minimalOptionsMap{flags.FlagHome: t.TempDir(), "mempool.type": "flood"}
	flood := cronos.New(log.NewNopLogger(), dbm.NewMemDB(), true, floodOpts, baseapp.SetChainID(cronos.TestAppChainID))
	t.Cleanup(func() { _ = flood.Close() })
	require.Nil(t, flood.MempoolManager())

	// No app mempool → MempoolClient() returns nil; ethermint then uses BroadcastTx.
	require.Nil(t, flood.MempoolClient())
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
