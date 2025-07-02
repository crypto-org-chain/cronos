package keeper_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/cometbft/cometbft/crypto/tmhash"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmversion "github.com/cometbft/cometbft/proto/tendermint/version"
	"github.com/cometbft/cometbft/version"
	"github.com/crypto-org-chain/cronos/v2/app"
	cronosmodulekeeper "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper"
	keepertest "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper/mock"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	ethermint "github.com/evmos/ethermint/types"
	evmtypes "github.com/evmos/ethermint/x/evm/types"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const (
	denom        = "testdenom"
	denomGravity = "gravity0x0000000000000000000000000000000000000000"
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

func (suite *KeeperTestSuite) DoSetupTest(t *testing.T) {
	t.Helper()
	// account key
	priv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	suite.address = common.BytesToAddress(priv.PubKey().Address().Bytes())

	// consensus key
	priv, err = ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	consAddress := sdk.ConsAddress(priv.PubKey().Address())

	suite.app = app.Setup(t, sdk.AccAddress(suite.address.Bytes()).String())
	blockIDHash := tmhash.Sum([]byte("block_id"))
	hash := tmhash.Sum([]byte("partset_header"))
	suite.ctx = suite.app.NewContext(false).WithBlockHeader(tmproto.Header{
		Height:          1,
		ChainID:         app.TestAppChainID,
		Time:            time.Now().UTC(),
		ProposerAddress: consAddress.Bytes(),
		Version: tmversion.Consensus{
			Block: version.BlockProtocol,
		},
		LastBlockId: tmproto.BlockID{
			Hash: blockIDHash,
			PartSetHeader: tmproto.PartSetHeader{
				Total: 11,
				Hash:  hash,
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

	acc.AccountNumber = suite.app.AccountKeeper.NextAccountNumber(suite.ctx)
	suite.app.AccountKeeper.SetAccount(suite.ctx, acc)

	valAddr := sdk.ValAddress(suite.address.Bytes())
	validator, err := stakingtypes.NewValidator(valAddr.String(), priv.PubKey(), stakingtypes.Description{})
	require.NoError(t, err)
	err = suite.app.StakingKeeper.SetValidatorByConsAddr(suite.ctx, validator)
	require.NoError(t, err)
	err = suite.app.StakingKeeper.SetValidatorByConsAddr(suite.ctx, validator)
	require.NoError(t, err)
	err = suite.app.StakingKeeper.SetValidator(suite.ctx, validator)
	require.NoError(t, err)

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

func (suite *KeeperTestSuite) RegisterSourceToken(
	contractAddress, symbol string, decimal uint32,
) error {
	denom := "cronos" + contractAddress
	msg := types.MsgUpdateTokenMapping{
		Denom:    denom,
		Contract: contractAddress,
		Symbol:   symbol,
		Decimal:  decimal,
	}
	return suite.app.CronosKeeper.RegisterOrUpdateTokenMapping(suite.ctx, &msg)
}

func (suite *KeeperTestSuite) TestDenomContractMap() {
	denom1 := denom + "1"
	denom2 := denom + "2"

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

				_, found := keeper.GetContractByDenom(suite.ctx, denom1)
				suite.Require().False(found)

				keeper.SetAutoContractForDenom(suite.ctx, denom1, autoContract)

				contract, found := keeper.GetContractByDenom(suite.ctx, denom1)
				suite.Require().True(found)
				suite.Require().Equal(autoContract, contract)

				err := keeper.SetExternalContractForDenom(suite.ctx, denom1, externalContract)
				suite.Require().NoError(err)

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
			sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123)), sdk.NewCoin("bad", sdkmath.NewInt(10))),
			func() {
				err := suite.MintCoins(address, sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123))))
				suite.Require().NoError(err)
				// Verify balance IBC coin pre operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdkmath.NewInt(123), ibcCroCoin.Amount)
				// Verify balance EVM coin pre operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdkmath.NewInt(0), evmCoin.Amount)
			},
			func() {
				// Verify balance IBC coin post operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdkmath.NewInt(123), ibcCroCoin.Amount)
				// Verify balance EVM coin post operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdkmath.NewInt(0), evmCoin.Amount)
			},
		},
		{
			"state committed upon success",
			sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123))),
			func() {
				err := suite.MintCoins(address, sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123))))
				suite.Require().NoError(err)
				// Verify balance IBC coin pre operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdkmath.NewInt(123), ibcCroCoin.Amount)
				// Verify balance EVM coin pre operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdkmath.NewInt(0), evmCoin.Amount)
			},
			func() {
				// Verify balance IBC coin post operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdkmath.NewInt(0), ibcCroCoin.Amount)
				// Verify balance EVM coin post operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdkmath.NewInt(1230000000000), evmCoin.Amount)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			// Create Cronos Keeper with mock transfer keeper
			cronosKeeper := *cronosmodulekeeper.NewKeeper(
				suite.app.EncodingConfig().Codec,
				suite.app.GetKey(types.StoreKey),
				suite.app.GetKey(types.MemStoreKey),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
				suite.app.EvmKeeper,
				suite.app.AccountKeeper,
				authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			)
			suite.app.CronosKeeper = cronosKeeper

			tc.malleate()
			suite.app.CronosKeeper.OnRecvVouchers(suite.ctx, tc.coins, address.String())
			tc.postCheck()
		})
	}
}

func (suite *KeeperTestSuite) TestRegisterOrUpdateTokenMapping() {
	contractAddress := "0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883"

	testCases := []struct {
		name     string
		msg      types.MsgUpdateTokenMapping
		malleate func()
		error    bool
	}{
		{
			"Non source token, no error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "gravity0xf6d4fecb1a6fb7c2ca350169a050d483bd87b883",
				Contract: contractAddress,
				Symbol:   "",
				Decimal:  0,
			},
			func() {
			},
			false,
		},
		{
			"No hex contract address, error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "gravity0xf6d4fecb1a6fb7c2ca350169a050d483bd87b883",
				Contract: "test",
				Symbol:   "",
				Decimal:  0,
			},
			func() {
			},
			true,
		},
		{
			"Non source token, no hex contract address, error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "cronos0xtest",
				Contract: "test",
				Symbol:   "",
				Decimal:  0,
			},
			func() {
			},
			true,
		},
		{
			"Non source token, already exists, no error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "gravity0xf6d4fecb1a6fb7c2ca350169a050d483bd87b883",
				Contract: "",
				Symbol:   "",
				Decimal:  0,
			},
			func() {
				err := suite.app.CronosKeeper.SetExternalContractForDenom(
					suite.ctx,
					"gravity0xf6d4fecb1a6fb7c2ca350169a050d483bd87b883",
					common.HexToAddress(contractAddress))
				suite.Require().NoError(err)
			},
			false,
		},
		{
			"Source token, invalid denom, error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "cronos0xf6d4fecb1a6fb7c2ca350169a050d483bd87b88@",
				Contract: contractAddress,
				Symbol:   "",
				Decimal:  0,
			},
			func() {
			},
			true,
		},
		{
			"Source token, denom correct, no error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "cronos0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883",
				Contract: contractAddress,
				Symbol:   "",
				Decimal:  0,
			},
			func() {
			},
			false,
		},
		{
			"Source token, denom correct with decimal, no error",
			types.MsgUpdateTokenMapping{
				Sender:   "",
				Denom:    "cronos0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883",
				Contract: contractAddress,
				Symbol:   "Test",
				Decimal:  6,
			},
			func() {
			},
			false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			// Create Cronos Keeper with mock transfer keeper
			cronosKeeper := *cronosmodulekeeper.NewKeeper(
				suite.app.EncodingConfig().Codec,
				suite.app.GetKey(types.StoreKey),
				suite.app.GetKey(types.MemStoreKey),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
				suite.app.EvmKeeper,
				suite.app.AccountKeeper,
				authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			)
			suite.app.CronosKeeper = cronosKeeper

			tc.malleate()
			err := suite.app.CronosKeeper.RegisterOrUpdateTokenMapping(suite.ctx, &tc.msg)
			if tc.error {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}
