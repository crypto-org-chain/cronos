package app

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"math/big"
	"math/rand"
	"testing"

	sdkmath "cosmossdk.io/math"
	dbm "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/evmos/ethermint/tests"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	memiavlstore "github.com/crypto-org-chain/cronos/store"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
)

// BenchmarkERC20Transfer benchmarks execution of standard erc20 token transfer transactions
func BenchmarkERC20Transfer(b *testing.B) {
	b.Run("memdb", func(b *testing.B) {
		db := dbm.NewMemDB()
		benchmarkERC20Transfer(b, b.TempDir(), db, AppOptionsMap{})
	})
	b.Run("leveldb", func(b *testing.B) {
		homePath := b.TempDir()
		db, err := dbm.NewGoLevelDB("application", homePath)
		require.NoError(b, err)
		benchmarkERC20Transfer(b, homePath, db, AppOptionsMap{})
	})
	b.Run("memiavl", func(b *testing.B) {
		benchmarkERC20Transfer(b, b.TempDir(), nil, AppOptionsMap{
			memiavlstore.FlagMemIAVL: true,
		})
	})
}

type TestAccount struct {
	Address common.Address
	Priv    cryptotypes.PrivKey
	Nonce   uint64
}

// pass `nil` to db to use memiavl
func benchmarkERC20Transfer(b *testing.B, homePath string, db dbm.DB, appOpts servertypes.AppOptions) {
	txsPerBlock := 5000
	accounts := 100
	gasPrice := big.NewInt(100000000000)
	bigZero := big.NewInt(0)
	encodingConfig := MakeEncodingConfig()
	app := New(log.NewNopLogger(), db, nil, true, true, map[int64]bool{}, homePath, 0, encodingConfig, appOpts, baseapp.SetChainID(TestAppChainID))
	defer app.Close()

	chainID := big.NewInt(777)
	ethSigner := ethtypes.LatestSignerForChainID(chainID)

	var testAccounts []TestAccount
	for i := 0; i < accounts; i++ {
		priv, err := ethsecp256k1.GenerateKey()
		require.NoError(b, err)
		address := common.BytesToAddress(priv.PubKey().Address().Bytes())
		testAccounts = append(testAccounts, TestAccount{Address: address, Priv: priv})
	}

	signTx := func(acc *TestAccount, msg *evmtypes.MsgEthereumTx) ([]byte, error) {
		msg.From = acc.Address.Bytes()
		if err := msg.Sign(ethSigner, tests.NewSigner(acc.Priv)); err != nil {
			return nil, err
		}
		tx, err := msg.BuildTx(encodingConfig.TxConfig.NewTxBuilder(), evmtypes.DefaultEVMDenom)
		if err != nil {
			return nil, err
		}
		return encodingConfig.TxConfig.TxEncoder()(tx)
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
	genesisState := NewDefaultGenesisState(encodingConfig.Codec)
	genesisState = genesisStateWithValSet(
		b,
		app,
		genesisState,
		valSet,
		accs,
		balances...,
	)

	appState, err := json.MarshalIndent(genesisState, "", "  ")
	require.NoError(b, err)

	blockParams := tmproto.BlockParams{
		MaxBytes: math.MaxInt64,
		MaxGas:   math.MaxInt64,
	}
	consensusParams := *DefaultConsensusParams
	consensusParams.Block = &blockParams
	app.InitChain(abci.RequestInitChain{
		ChainId:         TestAppChainID,
		AppStateBytes:   appState,
		ConsensusParams: &consensusParams,
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

	// mint to senders
	amount := int64(100000000)
	for _, acc := range testAccounts {
		_, err = app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "mint_by_cronos_module", acc.Address, big.NewInt(amount))
		require.NoError(b, err)
	}

	// check balance
	ret, err := app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "balanceOf", testAccounts[0].Address)
	require.NoError(b, err)
	require.Equal(b, uint64(amount), binary.BigEndian.Uint64(ret[32-8:]))

	app.EndBlock(abci.RequestEndBlock{})
	app.Commit()

	// prepare transactions
	var transferTxs [][]byte
	for i := 0; i < b.N; i++ {
		for j := 0; j < txsPerBlock; j++ {
			idx := rand.Int() % len(testAccounts)
			acct := &testAccounts[idx]
			recipient := common.BigToAddress(big.NewInt(int64(idx)))
			data, err := types.ModuleCRC21Contract.ABI.Pack("transfer", recipient, big.NewInt(1))
			require.NoError(b, err)

			tx := evmtypes.NewTx(
				chainID,
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

			bz, err := signTx(acct, tx)
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
			require.Equal(b, 0, int(res.Code), res.Log)
		}

		app.EndBlock(abci.RequestEndBlock{})
		app.Commit()
	}

	// check remaining balance, only check one account to avoid slow down benchmark itself
	ctx = app.NewContext(true, tmproto.Header{
		ChainID:         TestAppChainID,
		ProposerAddress: consAddress,
	})
	ret, err = app.CronosKeeper.CallModuleCRC21(ctx, contractAddr, "balanceOf", testAccounts[0].Address)
	require.NoError(b, err)
	require.Equal(b, uint64(amount)-testAccounts[0].Nonce, binary.BigEndian.Uint64(ret[32-8:]))
}
