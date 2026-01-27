package app

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	sdkmath "cosmossdk.io/math"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"

	evmtypes "github.com/evmos/ethermint/x/evm/types"
	feemarkettypes "github.com/evmos/ethermint/x/feemarket/types"
)

var (
	testSenderKey = mustDecodePrivKey("59c6995e998f97a5a0044975f0ca9f3d59c6995e998f97a5a0044975f0ca9f3d")
	testSender    = crypto.PubkeyToAddress(testSenderKey.PublicKey)
)

func TestNonceInconsistencyWithDisableTxReplacement(t *testing.T) {

	// Testnet reproduction: enabling --cronos.disable-tx-replacement forces
	// mempoolCacheMaxTxs = -1 (app.go lines 993-1004) so Ethermint's ante cache
	// never stores nonces. Before the v1.7 upgrade, validators already had Tx₁
	// (nonce=startNonce) from the account under test (the sender declared below)
	// in CheckTx, so CheckAndSetEthSenderNonce bumped the cached account sequence
	// to startNonce+1 even though the tx never hit a block. During the upgrade
	// the mempool was flushed, cache entries disappeared, but BaseApp rebuilt
	// check-state from the committed block so the sequence fell back to startNonce.
	//
	// This test mirrors that sequence of events without wiring the real cache:
	// - First CheckTx accepts the tx and bumps the in-memory sequence to startNonce+1
	// - A "restart" is simulated by explicitly resetting the account back to startNonce
	// - Second CheckTx re-accepts the tx (no cache entry) and bumps the sequence again
	// - Third CheckTx occurs immediately after and now fails with ErrInvalidSequence
	//   because the ante handler sees sequence=startNonce+1 but still has no cache
	//   evidence of a pending tx. The mempool is empty, yet replacement is impossible.
	app, _ := setupWithAppOptions(false, 0, map[string]interface{}{
		FlagDisableTxReplacement: true,
	})
	anteHandler := app.BaseApp.AnteHandler()

	sender := testSender
	recipient := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	const startNonce uint64 = 100

	deliverCtx := app.BaseApp.NewContext(true).
		WithBlockHeader(tmproto.Header{}).
		WithChainID(TestAppChainID).
		WithConsensusParams(*DefaultConsensusParams)
	accAddr := sdk.AccAddress(sender.Bytes())
	acc := app.AccountKeeper.NewAccountWithAddress(deliverCtx, accAddr)
	require.NoError(t, acc.SetSequence(startNonce))
	app.AccountKeeper.SetAccount(deliverCtx, acc)
	app.EvmKeeper.WithChainID(deliverCtx)
	evmParams := evmtypes.DefaultParams()
	evmParams.AllowUnprotectedTxs = true
	app.EvmKeeper.SetParams(deliverCtx, evmParams)
	app.FeeMarketKeeper.SetParams(deliverCtx, feemarkettypes.DefaultParams())
	app.FeeMarketKeeper.SetBaseFee(deliverCtx, big.NewInt(0))
	app.EvmKeeper.SetBalance(deliverCtx, sender, *uint256.NewInt(1_000_000_000_000_000), evmtypes.DefaultEVMDenom)

	tx := buildEthReplacementTx(t, app, recipient, startNonce)

	minGas := sdk.NewDecCoinFromDec(evmtypes.DefaultEVMDenom, sdkmath.LegacyNewDec(0))
	checkCtx := newCheckTxContext(app, minGas)
	_, err := anteHandler(checkCtx, tx, false)
	require.NoError(t, err)

	// reset sequence to simulate restart after upgrade
	deliverCtx = app.BaseApp.NewContext(true).
		WithBlockHeader(tmproto.Header{}).
		WithChainID(TestAppChainID).
		WithConsensusParams(*DefaultConsensusParams)
	acc = app.AccountKeeper.GetAccount(deliverCtx, accAddr)
	require.NotNil(t, acc)
	require.NoError(t, acc.SetSequence(startNonce))
	app.AccountKeeper.SetAccount(deliverCtx, acc)

	checkCtx = newCheckTxContext(app, minGas)
	_, err = anteHandler(checkCtx, tx, false)
	require.NoError(t, err)

	checkCtx = newCheckTxContext(app, minGas)
	_, err = anteHandler(checkCtx, tx, false)
	require.ErrorIs(t, err, errortypes.ErrInvalidSequence)
}

func buildEthReplacementTx(t *testing.T, app *App, to common.Address, nonce uint64) sdk.Tx {
	t.Helper()

	msg := evmtypes.NewTx(TestEthChainID, nonce, &to, big.NewInt(1), 21000, big.NewInt(1), nil, nil, nil, nil)
	signer := ethtypes.LatestSignerForChainID(TestEthChainID)
	signedTx, err := ethtypes.SignTx(msg.AsTransaction(), signer, testSenderKey)
	require.NoError(t, err)
	require.NoError(t, msg.FromSignedEthereumTx(signedTx, signer))

	tx, err := msg.BuildTx(app.TxConfig().NewTxBuilder(), evmtypes.DefaultEVMDenom)
	require.NoError(t, err)

	return tx
}

func mustDecodePrivKey(hexKey string) *ecdsa.PrivateKey {
	key, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		panic(err)
	}
	return key
}

func newCheckTxContext(app *App, minGas sdk.DecCoin) sdk.Context {
	return app.GetContextForCheckTx(nil).
		WithBlockHeader(tmproto.Header{}).
		WithChainID(TestAppChainID).
		WithConsensusParams(*DefaultConsensusParams).
		WithMinGasPrices(sdk.DecCoins{minGas})
}
