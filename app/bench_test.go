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

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	baseapp "github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	srvflags "github.com/evmos/ethermint/server/flags"
	"github.com/evmos/ethermint/tests"
	evmtypes "github.com/evmos/ethermint/x/evm/types"

	memiavlstore "github.com/crypto-org-chain/cronos/store"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
)

const BlockSTMPreEstimate = true

// BenchmarkERC20Transfer benchmarks execution of standard erc20 token transfer transactions
func BenchmarkERC20Transfer(b *testing.B) {
	b.Run("memdb", func(b *testing.B) {
		db := dbm.NewMemDB()
		benchmarkERC20Transfer(b, db, AppOptionsMap{
			flags.FlagHome: b.TempDir(),
		})
	})
	b.Run("leveldb", func(b *testing.B) {
		homePath := b.TempDir()
		db, err := dbm.NewDB("application", dbm.GoLevelDBBackend, filepath.Join(homePath, "data"))
		require.NoError(b, err)
		benchmarkERC20Transfer(b, db, AppOptionsMap{
			flags.FlagHome: homePath,
		})
	})
	b.Run("memiavl", func(b *testing.B) {
		benchmarkERC20Transfer(b, nil, AppOptionsMap{
			flags.FlagHome:           b.TempDir(),
			memiavlstore.FlagMemIAVL: true,
		})
	})
	for _, workers := range []int{1, 8, 16, 32} {
		b.Run(fmt.Sprintf("memiavl-stm-%d", workers), func(b *testing.B) {
			benchmarkERC20Transfer(b, nil, AppOptionsMap{
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

type TestAccount struct {
	Address common.Address
	Priv    cryptotypes.PrivKey
	Nonce   uint64
}

// pass `nil` to db to use memiavl
func benchmarkERC20Transfer(b *testing.B, db dbm.DB, appOpts servertypes.AppOptions) {
	txsPerBlock := 5000
	accounts := 100
	gasPrice := big.NewInt(100000000000)
	bigZero := big.NewInt(0)

	app := New(log.NewNopLogger(), db, nil, true, appOpts, baseapp.SetChainID(TestAppChainID))
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

	signTx := func(acc *TestAccount, msg *evmtypes.MsgEthereumTx) ([]byte, error) {
		msg.From = acc.Address.Bytes()
		if err := msg.Sign(ethSigner, tests.NewSigner(acc.Priv)); err != nil {
			return nil, err
		}
		tx, err := msg.BuildTx(app.TxConfig().NewTxBuilder(), evmtypes.DefaultEVMDenom)
		if err != nil {
			return nil, err
		}
		return app.TxConfig().TxEncoder()(tx)
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
	var transferTxs [][]byte
	for i := 0; i < b.N; i++ {
		for j := 0; j < txsPerBlock; j++ {
			idx := rand.Int() % len(testAccounts)
			acct := &testAccounts[idx]
			recipient := common.BigToAddress(big.NewInt(int64(idx)))
			data, err := types.ModuleCRC21Contract.ABI.Pack("transfer", recipient, big.NewInt(1))
			require.NoError(b, err)

			tx := evmtypes.NewTx(
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

			bz, err := signTx(acct, tx)
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
