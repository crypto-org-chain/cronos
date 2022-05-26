package keeper_test

import (
	cronosmodulekeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	keepertest "github.com/crypto-org-chain/cronos/x/cronos/keeper/mock"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"math/big"
	"testing"
	"time"

	evmtypes "github.com/tharsis/ethermint/x/evm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/version"
	"github.com/tharsis/ethermint/crypto/ethsecp256k1"
	ethermint "github.com/tharsis/ethermint/types"

	"github.com/crypto-org-chain/cronos/app"
)

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

type KeeperTestSuite struct {
	suite.Suite

	ctx sdk.Context
	app *app.App

	// EVM helpers
	evmParam evmtypes.Params

	address common.Address
}

func (suite *KeeperTestSuite) DoSetupTest(t require.TestingT) {
	checkTx := false

	// account key
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	suite.address = common.BytesToAddress(priv.PubKey().Address().Bytes())

	// consensus key
	priv, err = ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	consAddress := sdk.ConsAddress(priv.PubKey().Address())

	suite.app = app.Setup(checkTx, sdk.AccAddress(suite.address.Bytes()).String(), true)
	suite.ctx = suite.app.NewContext(checkTx, tmproto.Header{
		Height:          1,
		ChainID:         app.TestAppChainID,
		Time:            time.Now().UTC(),
		ProposerAddress: consAddress.Bytes(),
		Version: tmversion.Consensus{
			Block: version.BlockProtocol,
		},
		LastBlockId: tmproto.BlockID{
			Hash: tmhash.Sum([]byte("block_id")),
			PartSetHeader: tmproto.PartSetHeader{
				Total: 11,
				Hash:  tmhash.Sum([]byte("partset_header")),
			},
		},
		AppHash:            tmhash.Sum([]byte("app")),
		DataHash:           tmhash.Sum([]byte("data")),
		EvidenceHash:       tmhash.Sum([]byte("evidence")),
		ValidatorsHash:     tmhash.Sum([]byte("validators")),
		NextValidatorsHash: tmhash.Sum([]byte("next_validators")),
		ConsensusHash:      tmhash.Sum([]byte("consensus")),
		LastResultsHash:    tmhash.Sum([]byte("last_result")),
	})

	// initialize the validator
	acc := &ethermint.EthAccount{
		BaseAccount: authtypes.NewBaseAccount(sdk.AccAddress(suite.address.Bytes()), nil, 0, 0),
		CodeHash:    common.BytesToHash(ethcrypto.Keccak256(nil)).String(),
	}

	suite.app.AccountKeeper.SetAccount(suite.ctx, acc)

	valAddr := sdk.ValAddress(suite.address.Bytes())
	validator, err := stakingtypes.NewValidator(valAddr, priv.PubKey(), stakingtypes.Description{})
	err = suite.app.StakingKeeper.SetValidatorByConsAddr(suite.ctx, validator)
	require.NoError(t, err)
	err = suite.app.StakingKeeper.SetValidatorByConsAddr(suite.ctx, validator)
	require.NoError(t, err)
	suite.app.StakingKeeper.SetValidator(suite.ctx, validator)

	suite.evmParam = suite.app.EvmKeeper.GetParams(suite.ctx)
}

func (suite *KeeperTestSuite) SetupTest() {
	suite.DoSetupTest(suite.T())
}

func (suite *KeeperTestSuite) MintCoins(address sdk.AccAddress, coins sdk.Coins) error {
	err := suite.app.BankKeeper.MintCoins(suite.ctx, minttypes.ModuleName, coins)
	if err != nil {
		return err
	}
	err = suite.app.BankKeeper.SendCoinsFromModuleToAccount(suite.ctx, minttypes.ModuleName, address, coins)
	if err != nil {
		return err
	}
	return nil
}

func (suite *KeeperTestSuite) TestDenomContractMap() {
	denom1 := "testdenom1"
	denom2 := "testdenom2"

	autoContract := common.BigToAddress(big.NewInt(1))
	externalContract := common.BigToAddress(big.NewInt(2))

	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			"success, happy path",
			func() {
				keeper := suite.app.CronosKeeper

				contract, found := keeper.GetContractByDenom(suite.ctx, denom1)
				suite.Require().False(found)

				keeper.SetAutoContractForDenom(suite.ctx, denom1, autoContract)

				contract, found = keeper.GetContractByDenom(suite.ctx, denom1)
				suite.Require().True(found)
				suite.Require().Equal(autoContract, contract)

				keeper.SetExternalContractForDenom(suite.ctx, denom1, externalContract)

				contract, found = keeper.GetContractByDenom(suite.ctx, denom1)
				suite.Require().True(found)
				suite.Require().Equal(externalContract, contract)
			},
		},
		{
			"failure, multiple denoms map to same contract",
			func() {
				keeper := suite.app.CronosKeeper
				keeper.SetAutoContractForDenom(suite.ctx, denom1, autoContract)
				err := keeper.SetExternalContractForDenom(suite.ctx, denom2, autoContract)
				suite.Require().Error(err)
			},
		},
		{
			"failure, multiple denoms map to same external contract",
			func() {
				keeper := suite.app.CronosKeeper
				err := keeper.SetExternalContractForDenom(suite.ctx, denom1, externalContract)
				suite.Require().NoError(err)
				err = keeper.SetExternalContractForDenom(suite.ctx, denom2, externalContract)
				suite.Require().Error(err)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.malleate()
		})
	}
}

func (suite *KeeperTestSuite) MintCoinsToModule(module string, coins sdk.Coins) error {
	err := suite.app.BankKeeper.MintCoins(suite.ctx, module, coins)
	if err != nil {
		return err
	}
	return nil
}

func (suite *KeeperTestSuite) GetBalance(address sdk.AccAddress, denom string) sdk.Coin {
	return suite.app.BankKeeper.GetBalance(suite.ctx, address, denom)
}

func (suite *KeeperTestSuite) TestOnRecvVouchers() {
	privKey, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)
	address := sdk.AccAddress(privKey.PubKey().Address())

	testCases := []struct {
		name      string
		coins     sdk.Coins
		malleate  func()
		postCheck func()
	}{
		{
			"state reverted after error",
			sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdk.NewInt(123)), sdk.NewCoin("bad", sdk.NewInt(10))),
			func() {
				suite.MintCoins(address, sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdk.NewInt(123))))
				// Verify balance IBC coin pre operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdk.NewInt(123), ibcCroCoin.Amount)
				// Verify balance EVM coin pre operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdk.NewInt(0), evmCoin.Amount)
			},
			func() {
				// Verify balance IBC coin post operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdk.NewInt(123), ibcCroCoin.Amount)
				// Verify balance EVM coin post operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdk.NewInt(0), evmCoin.Amount)
			},
		},
		{
			"state committed upon success",
			sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdk.NewInt(123))),
			func() {
				suite.MintCoins(address, sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdk.NewInt(123))))
				// Verify balance IBC coin pre operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdk.NewInt(123), ibcCroCoin.Amount)
				// Verify balance EVM coin pre operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdk.NewInt(0), evmCoin.Amount)
			},
			func() {
				// Verify balance IBC coin post operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdk.NewInt(0), ibcCroCoin.Amount)
				// Verify balance EVM coin post operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdk.NewInt(1230000000000), evmCoin.Amount)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			// Create Cronos Keeper with mock transfer keeper
			cronosKeeper := *cronosmodulekeeper.NewKeeper(
				app.MakeEncodingConfig().Marshaler,
				suite.app.GetKey(types.StoreKey),
				suite.app.GetKey(types.MemStoreKey),
				suite.app.GetSubspace(types.ModuleName),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
				suite.app.GravityKeeper,
				suite.app.EvmKeeper,
				suite.app.AccountKeeper,
			)
			suite.app.CronosKeeper = cronosKeeper

			tc.malleate()
			suite.app.CronosKeeper.OnRecvVouchers(suite.ctx, tc.coins, address.String())
			tc.postCheck()
		})
	}
}
