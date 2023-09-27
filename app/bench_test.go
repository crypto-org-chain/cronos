package app

import (
	"encoding/binary"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	sdkmath "cosmossdk.io/math"
	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
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
		db, err := dbm.NewGoLevelDB("application", b.TempDir())
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
	var appOpts servertypes.AppOptions = EmptyAppOptions{}
	if db == nil {
		appOpts = AppOptionsMap(map[string]interface{}{
			memiavlstore.FlagMemIAVL: true,
		})
		require.NoError(b, os.RemoveAll(filepath.Join(DefaultNodeHome, "data/memiavl.db")))
	}
	encodingConfig := MakeEncodingConfig()
	app := New(log.NewNopLogger(), db, nil, true, true, map[int64]bool{}, DefaultNodeHome, 0, encodingConfig, appOpts, baseapp.SetChainID(TestAppChainID))
	defer app.Close()

	priv, err := ethsecp256k1.GenerateKey()
	address := common.BytesToAddress(priv.PubKey().Address().Bytes())
	signer := tests.NewSigner(priv)
	chainID := big.NewInt(777)
	ethSigner := ethtypes.LatestSignerForChainID(chainID)

	signTx := func(msg *evmtypes.MsgEthereumTx) ([]byte, error) {
		msg.From = address.Bytes()
		if err := msg.Sign(ethSigner, signer); err != nil {
			return nil, err
		}
		require.NoError(b, err)
		tx, err := msg.BuildTx(encodingConfig.TxConfig.NewTxBuilder(), evmtypes.DefaultEVMDenom)
		if err != nil {
			return nil, err
		}
		return encodingConfig.TxConfig.TxEncoder()(tx)
	}

	privVal := mock.NewPV()
	pubKey, err := privVal.GetPubKey()
	consAddress := sdk.ConsAddress(pubKey.Address())
	require.NoError(b, err)
	validator := tmtypes.NewValidator(pubKey, 1)
	valSet := tmtypes.NewValidatorSet([]*tmtypes.Validator{validator})
	acc := authtypes.NewBaseAccount(priv.PubKey().Address().Bytes(), priv.PubKey(), 0, 0)
	balance := banktypes.Balance{
		Address: acc.GetAddress().String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(evmtypes.DefaultEVMDenom, sdkmath.NewIntWithDecimal(10000000, 18))),
	}
	genesisState := NewDefaultGenesisState(encodingConfig.Codec)
	genesisState = genesisStateWithValSet(b, app, genesisState, valSet, []authtypes.GenesisAccount{acc}, balance)

	appState, err := json.MarshalIndent(genesisState, "", "  ")
	require.NoError(b, err)
	app.InitChain(abci.RequestInitChain{
		ChainId:         TestAppChainID,
		AppStateBytes:   appState,
		ConsensusParams: DefaultConsensusParams,
	})
	app.BeginBlock(abci.RequestBeginBlock{
		Header: tmproto.Header{
			Height:          1,
			ChainID:         TestAppChainID,
			ProposerAddress: consAddress,
		},
	})

	// deploy contract
	ctx := app.GetContextForDeliverTx(nil)
	contractAddr, err := app.CronosKeeper.DeployModuleCRC21(ctx, "test")
	require.NoError(b, err)

	// mint to sender
	amount := int64(100000000)
	_, err = app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "mint_by_cronos_module", address, big.NewInt(amount))
	require.NoError(b, err)

	// check balance
	ret, err := app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "balanceOf", address)
	require.NoError(b, err)
	require.Equal(b, uint64(amount), binary.BigEndian.Uint64(ret[32-8:]))

	app.EndBlock(abci.RequestEndBlock{})
	app.Commit()

	// prepare transactions
	var transferTxs [][]byte
	for i := 0; i < b.N; i++ {
		for j := 0; j < txsPerBlock; j++ {
			idx := int64(i*txsPerBlock + j)
			recipient := common.BigToAddress(big.NewInt(idx))
			data, err := types.ModuleCRC21Contract.ABI.Pack("transfer", recipient, big.NewInt(1))
			require.NoError(b, err)
			bz, err := signTx(evmtypes.NewTx(chainID, uint64(idx), &contractAddr, big.NewInt(0), 210000, gasPrice, nil, nil, data, nil))
			require.NoError(b, err)
			transferTxs = append(transferTxs, bz)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.BeginBlock(abci.RequestBeginBlock{
			Header: tmproto.Header{
				Height:          int64(i) + 2,
				ChainID:         TestAppChainID,
				ProposerAddress: consAddress,
			},
		})
		for j := 0; j < txsPerBlock; j++ {
			idx := i*txsPerBlock + j
			res := app.DeliverTx(abci.RequestDeliverTx{
				Tx: transferTxs[idx],
			})
			require.Equal(b, 0, int(res.Code))
		}

		// check remaining balance
		ctx := app.GetContextForDeliverTx(nil)
		ret, err = app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "balanceOf", address)
		require.NoError(b, err)
		require.Equal(b, uint64(amount)-uint64((i+1)*txsPerBlock), binary.BigEndian.Uint64(ret[32-8:]))

		app.EndBlock(abci.RequestEndBlock{})
		app.Commit()
	}
}
