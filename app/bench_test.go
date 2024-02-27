package app

import (
	"encoding/binary"
	"encoding/json"
	"math/big"
	"testing"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	memiavlstore "github.com/crypto-org-chain/cronos/store"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/evmos/ethermint/tests"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/require"
)

// BenchmarkERC20Transfer benchmarks execution of standard erc20 token transfer transactions
func BenchmarkERC20Transfer(b *testing.B) {
	b.Run("memdb", func(b *testing.B) {
		db := dbm.NewMemDB()
		benchmarkERC20Transfer(b, db)
	})
	b.Run("leveldb", func(b *testing.B) {
		db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, b.TempDir())
		require.NoError(b, err)
		benchmarkERC20Transfer(b, db)
	})
	b.Run("memiavl", func(b *testing.B) {
		benchmarkERC20Transfer(b, nil)
	})
}

// pass `nil` to db to use memiavl
func benchmarkERC20Transfer(b *testing.B, db dbm.DB) {
	txsPerBlock := 1000
	gasPrice := big.NewInt(100000000000)
	homePath := b.TempDir()
	appOpts := make(AppOptionsMap)
	appOpts[flags.FlagHome] = homePath
	if db == nil {
		appOpts[memiavlstore.FlagMemIAVL] = true
		appOpts[memiavlstore.FlagCacheSize] = 0
	}
	app := New(log.NewNopLogger(), db, nil, true, appOpts, baseapp.SetChainID(TestAppChainID))
	defer app.Close()

	priv, err := ethsecp256k1.GenerateKey()
	address := common.BytesToAddress(priv.PubKey().Address().Bytes())
	signer := tests.NewSigner(priv)
	ethSigner := ethtypes.LatestSignerForChainID(TestEthChainID)

	signTx := func(msg *evmtypes.MsgEthereumTx) ([]byte, error) {
		msg.From = address.Bytes()
		if err := msg.Sign(ethSigner, signer); err != nil {
			return nil, err
		}
		require.NoError(b, err)
		tx, err := msg.BuildTx(app.TxConfig().NewTxBuilder(), evmtypes.DefaultEVMDenom)
		if err != nil {
			return nil, err
		}
		return app.EncodingConfig().TxConfig.TxEncoder()(tx)
	}

	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	require.NoError(b, err)

	consAddress := sdk.ConsAddress(pubKey.Address())
	validator := tmtypes.NewValidator(pubKey, 1)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
	acc := authtypes.NewBaseAccount(priv.PubKey().Address().Bytes(), priv.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(evmtypes.DefaultEVMDenom, sdkmath.NewIntWithDecimal(10000000, 18))),
	}
	genesisState, err := simtestutil.GenesisStateWithValSet(
		app.AppCodec(),
		app.DefaultGenesis(),
		valSet,
		[]authtypes.GenesisAccount{acc},
		balance,
	)
	require.NoError(b, err)

	appState, err := json.MarshalIndent(genesisState, "", "  ")
	require.NoError(b, err)
	_, err = app.InitChain(&abci.RequestInitChain{
		ChainId:         TestAppChainID,
		AppStateBytes:   appState,
		ConsensusParams: DefaultConsensusParams,
	})
	require.NoError(b, err)

	_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{
		Height:          1,
		ProposerAddress: consAddress,
	})
	require.NoError(b, err)

	// deploy contract
	ctx := app.NewUncachedContext(false, cmtproto.Header{
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

		// mint to sender
		_, err = app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "mint_by_cronos_module", address, big.NewInt(amount))
		require.NoError(b, err)

		// check balance
		ret, err := app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "balanceOf", address)
		require.NoError(b, err)
		require.Equal(b, uint64(amount), binary.BigEndian.Uint64(ret[32-8:]))
		write()
	}

	_, err = app.Commit()
	require.NoError(b, err)

	// check remaining balance
	ctx = app.GetContextForCheckTx(nil)

	codeRsp, err := app.EvmKeeper.Code(ctx, &evmtypes.QueryCodeRequest{
		Address: contractAddr.Hex(),
	})
	require.NoError(b, err)
	require.NotEmpty(b, codeRsp.Code)

	// prepare transactions
	var transferTxs [][]byte
	for i := 0; i < b.N; i++ {
		for j := 0; j < txsPerBlock; j++ {
			idx := int64(i*txsPerBlock + j)
			recipient := common.BigToAddress(big.NewInt(idx))
			data, err := types.ModuleCRC21Contract.ABI.Pack("transfer", recipient, big.NewInt(1))
			require.NoError(b, err)
			bz, err := signTx(evmtypes.NewTx(TestEthChainID, uint64(idx), &contractAddr, big.NewInt(0), 210000, gasPrice, nil, nil, data, nil))
			require.NoError(b, err)
			transferTxs = append(transferTxs, bz)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rsp, err := app.FinalizeBlock(&abci.RequestFinalizeBlock{
			Txs:             transferTxs[i*txsPerBlock : (i+1)*txsPerBlock],
			Height:          int64(i) + 2,
			ProposerAddress: consAddress,
		})
		require.NoError(b, err)
		for _, txResult := range rsp.TxResults {
			require.Equal(b, abci.CodeTypeOK, txResult.Code)
		}
		_, err = app.Commit()
		require.NoError(b, err)

		// check remaining balance
		ctx := app.GetContextForCheckTx(nil)
		ret, err := app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "balanceOf", address)
		require.NoError(b, err)
		require.Equal(b, uint64(amount)-uint64((i+1)*txsPerBlock), binary.BigEndian.Uint64(ret[32-8:]))
	}
}
