package mempool_test

import (
	"context"
	"encoding/json"
	"maps"
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
	"github.com/stretchr/testify/suite"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"
	sdkmath "cosmossdk.io/math"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/server"
	"github.com/cosmos/cosmos-sdk/server/config"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
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
		flags.FlagHome:                 tb.TempDir(),
		cronos.FlagMempoolType:         "app",
		server.FlagMempoolMaxTxs:       100000,
		"cronos.mempool-tx-cache-size": 200000,
	}
	maps.Copy(appOpts, overrides)
	app := cronos.New(log.NewNopLogger(), dbm.NewMemDB(), true, appOpts, baseapp.SetChainID(cronos.TestAppChainID))
	tb.Cleanup(func() { _ = app.Close() })

	testAccounts := newTestAccounts(tb, accounts)

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

func newTestAccounts(tb testing.TB, n int) []testAccount {
	tb.Helper()
	accounts := make([]testAccount, n)
	for i := range accounts {
		priv, err := ethsecp256k1.GenerateKey()
		require.NoError(tb, err)
		accounts[i] = testAccount{
			Address: common.BytesToAddress(priv.PubKey().Address().Bytes()),
			Priv:    priv,
		}
	}
	return accounts
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

// The decode cache shares one wrapper for identical bytes, and encoding a wrapper with empty
// body/auth-info bytes writes them, so concurrent admissions race without encodeMu. Run with -race.
func TestConcurrentAdmissionOfEmptyTx(t *testing.T) {
	const goroutines = 16
	f := setupAdmissionApp(t, 1)
	bz, err := f.app.TxConfig().TxEncoder()(f.app.TxConfig().NewTxBuilder().GetTx())
	require.NoError(t, err)

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			resp, err := f.app.InsertTx(&abci.RequestInsertTx{Tx: bz})
			if err != nil || resp.Code == abci.CodeTypeOK {
				t.Errorf("a tx with no msgs must be rejected: err=%v code=%d", err, resp.Code)
			}
		})
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

// A nil pk leaves the SignerInfo without a public key, valid only because acc's key is already on-chain.
func (f *admissionFixture) signBankSend(tb testing.TB, acc *testAccount, pk cryptotypes.PubKey) *txtypes.TxRaw {
	tb.Helper()
	txCfg := f.app.TxConfig()
	from := sdk.AccAddress(acc.Address.Bytes())
	builder := txCfg.NewTxBuilder()
	require.NoError(tb, builder.SetMsgs(banktypes.NewMsgSend(from, from, sdk.NewCoins(sdk.NewInt64Coin(evmtypes.DefaultEVMDenom, 1)))))
	builder.SetGasLimit(200000)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(evmtypes.DefaultEVMDenom, sdkmath.NewIntWithDecimal(1, 16))))
	sig := signing.SignatureV2{
		PubKey:   pk,
		Data:     &signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_DIRECT},
		Sequence: acc.Nonce,
	}
	require.NoError(tb, builder.SetSignatures(sig))

	ctx := f.app.NewContext(true)
	signerData := authsigning.SignerData{
		Address:       from.String(),
		ChainID:       cronos.TestAppChainID,
		AccountNumber: f.app.AccountKeeper.GetAccount(ctx, from).GetAccountNumber(),
		Sequence:      acc.Nonce,
		PubKey:        acc.Priv.PubKey(),
	}
	signBytes, err := authsigning.GetSignBytesAdapter(ctx, txCfg.SignModeHandler(), signing.SignMode_SIGN_MODE_DIRECT, signerData, builder.GetTx())
	require.NoError(tb, err)
	sig.Data.(*signing.SingleSignatureData).Signature, err = acc.Priv.Sign(signBytes)
	require.NoError(tb, err)
	require.NoError(tb, builder.SetSignatures(sig))
	acc.Nonce++

	bz, err := txCfg.TxEncoder()(builder.GetTx())
	require.NoError(tb, err)
	var raw txtypes.TxRaw
	require.NoError(tb, raw.Unmarshal(bz))
	return &raw
}

// withSignerPubKey sets the first SignerInfo's public key, breaking the signature.
func withSignerPubKey(tb testing.TB, raw *txtypes.TxRaw, pk cryptotypes.PubKey) {
	tb.Helper()
	var authInfo txtypes.AuthInfo
	require.NoError(tb, authInfo.Unmarshal(raw.AuthInfoBytes))
	pkAny, err := codectypes.NewAnyWithValue(pk)
	require.NoError(tb, err)
	authInfo.SignerInfos[0].PublicKey = pkAny
	raw.AuthInfoBytes, err = authInfo.Marshal()
	require.NoError(tb, err)
}

func encodeRaw(tb testing.TB, raw *txtypes.TxRaw) []byte {
	tb.Helper()
	bz, err := raw.Marshal()
	require.NoError(tb, err)
	return bz
}

type SignerInfoTxTestSuite struct {
	suite.Suite
}

func TestSignerInfoTxTestSuite(t *testing.T) {
	suite.Run(t, new(SignerInfoTxTestSuite))
}

// withUnordered sets Body.Unordered, which an eth tx's signature doesn't cover.
func withUnordered(tb testing.TB, bz []byte) []byte {
	tb.Helper()
	var raw txtypes.TxRaw
	require.NoError(tb, raw.Unmarshal(bz))
	var body txtypes.TxBody
	require.NoError(tb, body.Unmarshal(raw.BodyBytes))
	body.Unordered = true
	var err error
	raw.BodyBytes, err = body.Marshal()
	require.NoError(tb, err)
	return encodeRaw(tb, &raw)
}

func (f *admissionFixture) finalizeTx(tb testing.TB, bz []byte) *abci.ExecTxResult {
	tb.Helper()
	req := &abci.RequestFinalizeBlock{Height: 2, ProposerAddress: f.consAddress, Txs: [][]byte{bz}}
	var resp *abci.ResponseFinalizeBlock
	var err error
	require.NotPanics(tb, func() { resp, err = f.app.FinalizeBlock(req) })
	require.NoError(tb, err)
	require.Len(tb, resp.TxResults, 1)
	return resp.TxResults[0]
}

// Only the app mempool rechecks itself; under flood, CometBFT rechecks each pooled tx.
func (f *admissionFixture) recheck(tb testing.TB, pooled []byte) {
	tb.Helper()
	if m := f.app.MempoolManager(); m != nil {
		m.WaitForRecheck(context.Background())
		return
	}
	_, err := f.app.CheckTx(&abci.RequestCheckTx{Tx: pooled, Type: abci.CheckTxType_Recheck})
	require.NoError(tb, err)
}

var (
	blockExecutors = []string{config.BlockExecutorSequential, config.BlockExecutorBlockSTM}
	// Validators may run any of these, so a block tx's result must not depend on which.
	resultParityMempools = []struct {
		name   string
		opts   minimalOptionsMap
		pooled bool
	}{
		{name: "app", pooled: true},
		{name: "flood", opts: minimalOptionsMap{cronos.FlagMempoolType: ""}, pooled: true},
		{name: "noop", opts: minimalOptionsMap{cronos.FlagMempoolType: "", server.FlagMempoolMaxTxs: -1}},
	}
)

// A block can carry any decodable tx; FinalizeBlock must not panic, and the result must not
// depend on the node's mempool config.
func (s *SignerInfoTxTestSuite) TestFinalizeBlockDoesNotPanic() {
	testCases := []struct {
		name     string
		malleate func(raw *txtypes.TxRaw)
		expErr   *errorsmod.Error // nil means the msgs ran
		// expStaleEvicted: the block tx used the pooled tx's nonce, so the pool drops the pooled tx.
		expStaleEvicted bool
	}{
		{
			name:            "validly signed without a public key",
			malleate:        func(*txtypes.TxRaw) {},
			expStaleEvicted: true,
		},
		{
			name:     "forged signature without a public key",
			malleate: func(raw *txtypes.TxRaw) { raw.Signatures[0] = []byte("forged") },
			expErr:   sdkerrors.ErrUnauthorized,
		},
		{
			name: "malformed public key",
			malleate: func(raw *txtypes.TxRaw) {
				withSignerPubKey(s.T(), raw, &secp256k1.PubKey{Key: []byte{1}})
			},
			expErr: sdkerrors.ErrPanic, // ante panics deriving the key's address, on every node
		},
		{
			name: "no fee",
			malleate: func(raw *txtypes.TxRaw) {
				var authInfo txtypes.AuthInfo
				s.Require().NoError(authInfo.Unmarshal(raw.AuthInfoBytes))
				authInfo.Fee = nil
				var err error
				raw.AuthInfoBytes, err = authInfo.Marshal()
				s.Require().NoError(err)
			},
			expErr: sdkerrors.ErrPanic, // decodes, but every signer lookup dereferences the fee
		},
	}

	for _, executor := range blockExecutors {
		for _, mp := range resultParityMempools {
			for _, tc := range testCases {
				s.Run(executor+"/"+mp.name+"/"+tc.name, func() {
					opts := minimalOptionsMap{server.FlagBlockExecutor: executor}
					maps.Copy(opts, mp.opts)
					f := setupAdmissionAppWithOpts(s.T(), 1, opts)
					acc := &f.accounts[0]
					raw := f.signBankSend(s.T(), acc, nil)
					tc.malleate(raw)
					acc.Nonce = 0
					pooled := f.signTransfer(s.T(), acc, nil)
					checkResp, err := f.app.CheckTx(&abci.RequestCheckTx{Tx: pooled})
					s.Require().NoError(err)
					s.Require().Equal(abci.CodeTypeOK, checkResp.Code)

					res := f.finalizeTx(s.T(), encodeRaw(s.T(), raw))
					expCode, expCodespace := abci.CodeTypeOK, ""
					if tc.expErr != nil {
						expCode, expCodespace = tc.expErr.ABCICode(), tc.expErr.Codespace()
					}
					s.Require().Equal(expCode, res.Code, res.Log)
					s.Require().Equal(expCodespace, res.Codespace)
					_, err = f.app.Commit()
					s.Require().NoError(err)
					if !mp.pooled {
						return
					}
					f.recheck(s.T(), pooled)
					s.Require().Equal(tc.expStaleEvicted, f.app.Mempool().CountTx() == 0)
				})
			}
		}
	}
}

// runTx fails a tx whose finalize-mode removal errors, which a NoOp mempool never does.
func (s *SignerInfoTxTestSuite) TestFinalizeBlockRemovalErrorKeepsResult() {
	for _, executor := range blockExecutors {
		for _, mp := range resultParityMempools {
			s.Run(executor+"/"+mp.name, func() {
				opts := minimalOptionsMap{server.FlagBlockExecutor: executor}
				maps.Copy(opts, mp.opts)
				f := setupAdmissionAppWithOpts(s.T(), 1, opts)
				pooled := f.signTransfer(s.T(), &f.accounts[0], nil)
				checkResp, err := f.app.CheckTx(&abci.RequestCheckTx{Tx: pooled})
				s.Require().NoError(err)
				s.Require().Equal(abci.CodeTypeOK, checkResp.Code, checkResp.Log)

				// Unordered without a timeout timestamp, so the pool can't derive its removal key.
				res := f.finalizeTx(s.T(), withUnordered(s.T(), pooled))
				s.Require().Equal(abci.CodeTypeOK, res.Code, res.Log)
				_, err = f.app.Commit()
				s.Require().NoError(err)
				if !mp.pooled {
					return
				}
				// The pooled original's nonce is now spent, so recheck must evict it.
				f.recheck(s.T(), pooled)
				s.Require().Zero(f.app.Mempool().CountTx())
			})
		}
	}
}

// Transitional: a pooling node must not admit, and so never proposes, a tx without a public key;
// a block that carries one still runs it.
func (s *SignerInfoTxTestSuite) TestNoPubKeyTxNotAdmitted() {
	for _, mp := range resultParityMempools {
		if !mp.pooled {
			continue
		}
		s.Run(mp.name, func() {
			f := setupAdmissionAppWithOpts(s.T(), 1, mp.opts)
			bz := encodeRaw(s.T(), f.signBankSend(s.T(), &f.accounts[0], nil))
			checkResp, err := f.app.CheckTx(&abci.RequestCheckTx{Tx: bz})
			s.Require().NoError(err)
			// The pool's SDK adapter panics in Insert, as on unpatched nodes.
			s.Require().Equal(sdkerrors.ErrPanic.ABCICode(), checkResp.Code, checkResp.Log)
			s.Require().Equal(sdkerrors.ErrPanic.Codespace(), checkResp.Codespace)
			s.Require().Zero(f.app.Mempool().CountTx())

			res := f.finalizeTx(s.T(), bz)
			s.Require().Equal(abci.CodeTypeOK, res.Code, res.Log)
		})
	}
}

// The pool keys txs with the SDK adapter while staging uses the msg adapter, so a pooled cosmos
// tx must be dropped whether the block carries it or another tx at its nonce.
func (s *SignerInfoTxTestSuite) TestPooledCosmosTxLeavesPool() {
	testCases := []struct {
		name    string
		blockTx func(f *admissionFixture, pooled []byte) []byte
	}{
		{
			name:    "block carries the pooled tx",
			blockTx: func(_ *admissionFixture, pooled []byte) []byte { return pooled },
		},
		{
			name: "block carries another tx at its nonce, without a public key",
			blockTx: func(f *admissionFixture, _ []byte) []byte {
				acc := &f.accounts[0]
				acc.Nonce = 0
				return encodeRaw(s.T(), f.signBankSend(s.T(), acc, nil))
			},
		},
	}

	for _, mp := range resultParityMempools {
		if !mp.pooled {
			continue
		}
		for _, tc := range testCases {
			s.Run(mp.name+"/"+tc.name, func() {
				f := setupAdmissionAppWithOpts(s.T(), 1, mp.opts)
				acc := &f.accounts[0]
				pooled := encodeRaw(s.T(), f.signBankSend(s.T(), acc, acc.Priv.PubKey()))
				checkResp, err := f.app.CheckTx(&abci.RequestCheckTx{Tx: pooled})
				s.Require().NoError(err)
				s.Require().Equal(abci.CodeTypeOK, checkResp.Code, checkResp.Log)
				s.Require().Equal(1, f.app.Mempool().CountTx())

				res := f.finalizeTx(s.T(), tc.blockTx(f, pooled))
				s.Require().Equal(abci.CodeTypeOK, res.Code, res.Log)
				_, err = f.app.Commit()
				s.Require().NoError(err)
				f.recheck(s.T(), pooled)
				s.Require().Zero(f.app.Mempool().CountTx())
			})
		}
	}
}

func (s *SignerInfoTxTestSuite) TestMsgSignerExtractionAdapter() {
	f := setupAdmissionApp(s.T(), 1)
	acc := &f.accounts[0]
	from := sdk.AccAddress(acc.Address.Bytes())

	testCases := []struct {
		name     string
		malleate func(raw *txtypes.TxRaw)
		expErr   bool
		// expSDKParity: same signers as the SDK adapter this replaces.
		expSDKParity bool
	}{
		{
			name:         "public key present",
			malleate:     func(raw *txtypes.TxRaw) { withSignerPubKey(s.T(), raw, acc.Priv.PubKey()) },
			expSDKParity: true,
		},
		{
			name:     "no public key",
			malleate: func(*txtypes.TxRaw) {},
		},
		{
			name: "malformed public key",
			malleate: func(raw *txtypes.TxRaw) {
				withSignerPubKey(s.T(), raw, &secp256k1.PubKey{Key: []byte{1}})
			},
		},
		{
			name: "more signer infos than msg signers",
			malleate: func(raw *txtypes.TxRaw) {
				var authInfo txtypes.AuthInfo
				s.Require().NoError(authInfo.Unmarshal(raw.AuthInfoBytes))
				authInfo.SignerInfos = append(authInfo.SignerInfos, authInfo.SignerInfos[0])
				var err error
				raw.AuthInfoBytes, err = authInfo.Marshal()
				s.Require().NoError(err)
				raw.Signatures = append(raw.Signatures, raw.Signatures[0])
			},
			expErr: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			acc.Nonce = 0
			raw := f.signBankSend(s.T(), acc, nil)
			tc.malleate(raw)
			tx, err := f.app.TxConfig().TxDecoder()(encodeRaw(s.T(), raw))
			s.Require().NoError(err)

			var signers []sdkmempool.SignerData
			s.Require().NotPanics(func() { signers, err = mempool.NewMsgSignerExtractionAdapter().GetSigners(tx) })
			if tc.expErr {
				s.Require().Error(err)
				return
			}
			s.Require().NoError(err)
			s.Require().Equal([]sdkmempool.SignerData{sdkmempool.NewSignerData(from, 0)}, signers)
			if tc.expSDKParity {
				sdkSigners, err := sdkmempool.NewDefaultSignerExtractionAdapter().GetSigners(tx)
				s.Require().NoError(err)
				s.Require().Equal(sdkSigners, signers)
			}
		})
	}
}

func TestAppMempoolClientNilWithoutAppMempool(t *testing.T) {
	appMempool := setupAdmissionApp(t, 1).app
	require.NotNil(t, appMempool.MempoolManager())
	require.NotNil(t, appMempool.MempoolClient(), "app mempool exposes a client")

	floodOpts := minimalOptionsMap{flags.FlagHome: t.TempDir(), cronos.FlagMempoolType: "flood"}
	flood := cronos.New(log.NewNopLogger(), dbm.NewMemDB(), true, floodOpts, baseapp.SetChainID(cronos.TestAppChainID))
	t.Cleanup(func() { _ = flood.Close() })
	require.Nil(t, flood.MempoolManager())

	// No app mempool → MempoolClient() returns nil; ethermint then uses BroadcastTx.
	require.Nil(t, flood.MempoolClient())
}

// BenchmarkAdmission measures InsertTx throughput at the current single-mutex
// ceiling (Phase-2 baseline / GATE). Pre-verify runs lock-free; RunTx is
// serialized by the admission mutex.
func BenchmarkAdmission(b *testing.B) {
	const goroutines = 16

	testCases := []struct {
		name     string
		unfunded bool // senders aren't in genesis: txs pass pre-verify and decode, then ante rejects them
	}{
		{name: "valid"},
		{name: "rejected", unfunded: true},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Room for every tx, so a long -benchtime can't fill the pool and fail the admitted count.
			total := goroutines * b.N
			f := setupAdmissionAppWithOpts(b, goroutines, minimalOptionsMap{server.FlagMempoolMaxTxs: max(total, 100000)})
			senders := f.accounts
			if tc.unfunded {
				senders = newTestAccounts(b, goroutines)
			}

			// Pre-sign b.N txs per goroutine (distinct accounts → independent nonces).
			txs := make([][][]byte, goroutines)
			for g := range goroutines {
				txs[g] = make([][]byte, b.N)
				for i := range b.N {
					txs[g][i] = f.signTransfer(b, &senders[g], nil)
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

			wantAdmitted := int64(total)
			if tc.unfunded {
				wantAdmitted = 0
			}
			require.Equal(b, wantAdmitted, admitted.Load())
			if secs := b.Elapsed().Seconds(); secs > 0 {
				b.ReportMetric(float64(total)/secs, "tx/s")
			}
		})
	}
}
