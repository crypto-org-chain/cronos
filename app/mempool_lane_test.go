package app

import (
	"bytes"
	"math/big"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/evmos/ethermint/ante/interfaces"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	evmenc "github.com/evmos/ethermint/encoding"
	evmapp "github.com/evmos/ethermint/evmd"
	"github.com/evmos/ethermint/tests"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

type ethTestAccount struct {
	ethAddress common.Address
	sdkAddress sdk.AccAddress
	privKey    cryptotypes.PrivKey
}

func newEthTestAccount(t *testing.T) ethTestAccount {
	t.Helper()

	privKey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)

	ethAddr := common.BytesToAddress(privKey.PubKey().Address().Bytes())
	return ethTestAccount{
		ethAddress: ethAddr,
		sdkAddress: sdk.AccAddress(ethAddr.Bytes()),
		privKey:    privKey,
	}
}

func newSignedEthMsg(
	t *testing.T,
	ethSigner ethtypes.Signer,
	chainID *big.Int,
	acc ethTestAccount,
	nonce uint64,
	gasPrice int64,
) *evmtypes.MsgEthereumTx {
	t.Helper()

	to := common.BigToAddress(big.NewInt(1))
	msg := evmtypes.NewTx(chainID, nonce, &to, big.NewInt(1), 100_000, big.NewInt(gasPrice), nil, nil, nil, nil)
	msg.From = acc.sdkAddress.Bytes()
	require.NoError(t, msg.Sign(ethSigner, tests.NewSigner(acc.privKey)))
	return msg
}

func buildEthEnvelopeTx(t *testing.T, msgs ...*evmtypes.MsgEthereumTx) sdk.Tx {
	t.Helper()
	return buildEthEnvelopeTxWithDenom(t, evmtypes.DefaultEVMDenom, msgs...)
}

func buildEthEnvelopeTxWithDenom(t *testing.T, denom string, msgs ...*evmtypes.MsgEthereumTx) sdk.Tx {
	t.Helper()
	require.NotEmpty(t, msgs)

	txCfg := evmenc.MakeConfig().TxConfig
	txBuilder := txCfg.NewTxBuilder()
	extBuilder, ok := txBuilder.(authtx.ExtensionOptionsTxBuilder)
	require.True(t, ok)

	option, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
	require.NoError(t, err)
	extBuilder.SetExtensionOptions(option)

	sdkMsgs := make([]sdk.Msg, 0, len(msgs))
	totalFee := sdk.NewCoins()
	var totalGas uint64
	for _, msg := range msgs {
		sdkMsgs = append(sdkMsgs, msg)
		totalGas += msg.GetGas()
		totalFee = totalFee.Add(sdk.NewCoin(
			denom,
			sdkmath.NewIntFromBigInt(msg.GetFee()),
		))
	}

	require.NoError(t, extBuilder.SetMsgs(sdkMsgs...))
	extBuilder.SetFeeAmount(totalFee)
	extBuilder.SetGasLimit(totalGas)
	return txBuilder.GetTx()
}

func mustEncodeTx(t *testing.T, tx sdk.Tx) []byte {
	t.Helper()
	bz, err := evmenc.MakeConfig().TxConfig.TxEncoder()(tx)
	require.NoError(t, err)
	return bz
}

func newLaneTestMempool(feeBump int64) *mempool.MultiLanePriorityNonceMempool[int64] {
	return mempool.NewMultiLanePriorityMempool(mempool.PriorityNonceMempoolConfig[int64]{
		TxPriority:      mempool.NewDefaultTxPriority(),
		SignerExtractor: evmapp.NewEthSignerExtractionAdapter(mempool.NewDefaultSignerExtractionAdapter()),
		TxReplacement: func(op, np int64, _, _ sdk.Tx) bool {
			threshold := 100 + feeBump
			return np >= op*threshold/100
		},
	})
}

func TestAppUsesMultiLanePriorityMempool(t *testing.T) {
	app := Setup(t, sdk.AccAddress(newEthTestAccount(t).ethAddress.Bytes()).String())

	_, ok := app.Mempool().(*mempool.MultiLanePriorityNonceMempool[int64])
	require.True(t, ok)
}

func TestMempoolLaneBypassForHiddenSecondSigner(t *testing.T) {
	attacker := newEthTestAccount(t)
	victim := newEthTestAccount(t)
	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)

	batchTx := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, attacker, 1, 2),
		newSignedEthMsg(t, ethSigner, TestEthChainID, victim, 7, 2),
	)
	cancelTx := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, victim, 7, 3),
	)

	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger())
	pool := newLaneTestMempool(0)

	require.NoError(t, pool.Insert(ctx.WithPriority(100), batchTx))
	require.NoError(t, pool.Insert(ctx.WithPriority(101), cancelTx))

	require.Equal(t, 1, pool.CountTx())
	require.Nil(t, pool.NextSenderTx(attacker.sdkAddress.String()))
	require.True(t, bytes.Equal(
		mustEncodeTx(t, cancelTx),
		mustEncodeTx(t, pool.NextSenderTx(victim.sdkAddress.String())),
	))
}

func TestMempoolLaneReplacementEvictsBatchWhenConflictIsThirdInnerSigner(t *testing.T) {
	accA := newEthTestAccount(t)
	accB := newEthTestAccount(t)
	victim := newEthTestAccount(t)
	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)

	batchTx := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, accA, 1, 2),
		newSignedEthMsg(t, ethSigner, TestEthChainID, accB, 5, 2),
		newSignedEthMsg(t, ethSigner, TestEthChainID, victim, 7, 2),
	)
	cancelTx := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, victim, 7, 3),
	)

	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger())
	pool := newLaneTestMempool(0)
	require.NoError(t, pool.Insert(ctx.WithPriority(100), batchTx))
	require.NoError(t, pool.Insert(ctx.WithPriority(101), cancelTx))

	require.Equal(t, 1, pool.CountTx())
	require.Nil(t, pool.NextSenderTx(accA.sdkAddress.String()))
	require.Nil(t, pool.NextSenderTx(accB.sdkAddress.String()))
	require.True(t, bytes.Equal(
		mustEncodeTx(t, cancelTx),
		mustEncodeTx(t, pool.NextSenderTx(victim.sdkAddress.String())),
	))
}

func TestLaneReplacementFeeBumpPerLane(t *testing.T) {
	senderA := newEthTestAccount(t)
	senderB := newEthTestAccount(t)
	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)

	txA := buildEthEnvelopeTx(t, newSignedEthMsg(t, ethSigner, TestEthChainID, senderA, 1, 2))
	txB := buildEthEnvelopeTx(t, newSignedEthMsg(t, ethSigner, TestEthChainID, senderB, 2, 2))
	candidateLow := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, senderA, 1, 3),
		newSignedEthMsg(t, ethSigner, TestEthChainID, senderB, 2, 3),
	)
	candidateHigh := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, senderA, 1, 4),
		newSignedEthMsg(t, ethSigner, TestEthChainID, senderB, 2, 4),
	)

	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger())
	pool := newLaneTestMempool(10)

	require.NoError(t, pool.Insert(ctx.WithPriority(100), txA))
	require.NoError(t, pool.Insert(ctx.WithPriority(10), txB))

	// Replacement uses the envelope's overall priority (max across all lanes = 100)
	// as the baseline, so the 10% bump threshold is 110 for ALL lanes. candidateLow
	// (priority 109) fails even though it would satisfy lane B's individual threshold
	// (10 * 110/100 = 11). This is intentional all-or-nothing cross-lane semantics.
	err := pool.Insert(ctx.WithPriority(109), candidateLow)
	require.Error(t, err)
	require.Contains(t, err.Error(), "replacement rule")
	require.Equal(t, 2, pool.CountTx())

	require.NoError(t, pool.Insert(ctx.WithPriority(110), candidateHigh))
	require.Equal(t, 1, pool.CountTx())
	require.True(t, bytes.Equal(
		mustEncodeTx(t, candidateHigh),
		mustEncodeTx(t, pool.NextSenderTx(senderA.sdkAddress.String())),
	))
	require.True(t, bytes.Equal(
		mustEncodeTx(t, candidateHigh),
		mustEncodeTx(t, pool.NextSenderTx(senderB.sdkAddress.String())),
	))
}

func TestDuplicateInnerLaneRejected(t *testing.T) {
	acc := newEthTestAccount(t)
	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)
	tx := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, acc, 7, 2),
		newSignedEthMsg(t, ethSigner, TestEthChainID, acc, 7, 3),
	)

	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger())
	params := evmtypes.DefaultParams()
	err := interfaces.ValidateEthBasic(ctx, tx, &params, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate inner ethereum lane")
}

func TestInnerMsgCap(t *testing.T) {
	acc := newEthTestAccount(t)
	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)
	tx := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, acc, 1, 2),
		newSignedEthMsg(t, ethSigner, TestEthChainID, acc, 2, 2),
		newSignedEthMsg(t, ethSigner, TestEthChainID, acc, 3, 2),
	)

	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger())
	params := evmtypes.DefaultParams()
	params.MaxEthMsgsPerTx = 2
	err := interfaces.ValidateEthBasic(ctx, tx, &params, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "number of messages should be <=")
}

func TestInnerMsgCapBoundaryAtLimitAccepted(t *testing.T) {
	acc := newEthTestAccount(t)
	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)
	tx := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, acc, 1, 2),
		newSignedEthMsg(t, ethSigner, TestEthChainID, acc, 2, 2),
	)

	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger())
	params := evmtypes.DefaultParams()
	params.MaxEthMsgsPerTx = 2
	require.NoError(t, interfaces.ValidateEthBasic(ctx, tx, &params, nil))
}

func TestInnerMsgCapZeroUsesDefaultCap(t *testing.T) {
	acc := newEthTestAccount(t)
	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)
	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger())
	params := evmtypes.DefaultParams()
	params.MaxEthMsgsPerTx = 0

	msgs64 := make([]*evmtypes.MsgEthereumTx, 0, evmtypes.DefaultMaxEthMsgsPerTx)
	for i := range evmtypes.DefaultMaxEthMsgsPerTx {
		msgs64 = append(msgs64, newSignedEthMsg(t, ethSigner, TestEthChainID, acc, uint64(i+1), 2))
	}
	tx64 := buildEthEnvelopeTx(t, msgs64...)
	require.NoError(t, interfaces.ValidateEthBasic(ctx, tx64, &params, nil))

	// Pre-allocate cap=len+1 so the append below always triggers a new backing
	// array regardless of Go's growth heuristics.
	msgs65 := make([]*evmtypes.MsgEthereumTx, len(msgs64), len(msgs64)+1)
	copy(msgs65, msgs64)
	msgs65 = append(msgs65, newSignedEthMsg(t, ethSigner, TestEthChainID, acc, uint64(len(msgs65)+1), 2))
	tx65 := buildEthEnvelopeTx(t, msgs65...)
	err := interfaces.ValidateEthBasic(ctx, tx65, &params, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "number of messages should be <=")
}

func TestInnerMsgCapGovernanceUpdate(t *testing.T) {
	admin := newEthTestAccount(t)
	app := Setup(t, admin.sdkAddress.String())

	ctx := app.NewUncachedContext(false, cmtproto.Header{ChainID: TestAppChainID})
	chainID := app.EvmKeeper.ChainID()
	ethSigner := ethtypes.LatestSignerForChainID(chainID)
	acc := newEthTestAccount(t)
	params := app.EvmKeeper.GetParams(ctx)
	params.EnableCreate = true
	params.EnableCall = true
	denom := params.EvmDenom
	if denom == "" {
		denom = evmtypes.DefaultEVMDenom
	}
	params.EvmDenom = denom
	tx := buildEthEnvelopeTxWithDenom(t, denom,
		newSignedEthMsg(t, ethSigner, chainID, acc, 1, 2),
		newSignedEthMsg(t, ethSigner, chainID, acc, 2, 2),
		newSignedEthMsg(t, ethSigner, chainID, acc, 3, 2),
	)
	baseFee := app.EvmKeeper.GetBaseFee(ctx, params.GetChainConfig().EthereumConfig(chainID))
	require.NoError(t, interfaces.ValidateEthBasic(ctx, tx, &params, baseFee))

	params.MaxEthMsgsPerTx = 2
	authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
	_, err := app.EvmKeeper.UpdateParams(ctx, &evmtypes.MsgUpdateParams{
		Authority: authority,
		Params:    params,
	})
	require.NoError(t, err)

	updated := app.EvmKeeper.GetParams(ctx)
	require.Equal(t, uint32(2), updated.MaxEthMsgsPerTx)
	baseFee = app.EvmKeeper.GetBaseFee(ctx, updated.GetChainConfig().EthereumConfig(chainID))
	err = interfaces.ValidateEthBasic(ctx, tx, &updated, baseFee)
	require.Error(t, err)
	require.Contains(t, err.Error(), "number of messages should be <=")
}

func TestDuplicateInnerLaneNonAdjacentRejected(t *testing.T) {
	accA := newEthTestAccount(t)
	accB := newEthTestAccount(t)
	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)
	tx := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, accA, 7, 2),
		newSignedEthMsg(t, ethSigner, TestEthChainID, accB, 1, 2),
		newSignedEthMsg(t, ethSigner, TestEthChainID, accA, 7, 3),
	)

	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger())
	params := evmtypes.DefaultParams()
	err := interfaces.ValidateEthBasic(ctx, tx, &params, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate inner ethereum lane")
}

func TestDistinctSignersSameNonceAreNotDuplicateLanes(t *testing.T) {
	accA := newEthTestAccount(t)
	accB := newEthTestAccount(t)
	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)
	tx := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, accA, 7, 2),
		newSignedEthMsg(t, ethSigner, TestEthChainID, accB, 7, 2),
	)

	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger())
	params := evmtypes.DefaultParams()
	require.NoError(t, interfaces.ValidateEthBasic(ctx, tx, &params, nil))

	pool := newLaneTestMempool(0)
	require.NoError(t, pool.Insert(ctx.WithPriority(100), tx))
	require.Equal(t, 1, pool.CountTx())
	require.True(t, bytes.Equal(
		mustEncodeTx(t, tx),
		mustEncodeTx(t, pool.NextSenderTx(accA.sdkAddress.String())),
	))
	require.True(t, bytes.Equal(
		mustEncodeTx(t, tx),
		mustEncodeTx(t, pool.NextSenderTx(accB.sdkAddress.String())),
	))
}

func TestBatchSelectionEmitsEachBatchOnce(t *testing.T) {
	accA := newEthTestAccount(t)
	accB := newEthTestAccount(t)
	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)

	batchTx := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, accA, 1, 2),
		newSignedEthMsg(t, ethSigner, TestEthChainID, accB, 4, 2),
	)
	standalone := buildEthEnvelopeTx(t,
		newSignedEthMsg(t, ethSigner, TestEthChainID, accA, 2, 2),
	)

	ctx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger())
	pool := newLaneTestMempool(0)
	require.NoError(t, pool.Insert(ctx.WithPriority(100), batchTx))
	require.NoError(t, pool.Insert(ctx.WithPriority(90), standalone))

	batchBytes := mustEncodeTx(t, batchTx)
	batchSelected := 0
	iter := pool.Select(ctx, nil)
	for iter != nil {
		// iter.Tx() returns a mempool.WrappedTx; .Tx is the inner sdk.Tx.
		if bytes.Equal(batchBytes, mustEncodeTx(t, iter.Tx().Tx)) {
			batchSelected++
		}
		iter = iter.Next()
	}

	require.Equal(t, 1, batchSelected)
}
