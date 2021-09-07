package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/app"
	cronosmodulekeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	keepertest "github.com/crypto-org-chain/cronos/x/cronos/keeper/mock"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/tharsis/ethermint/crypto/ethsecp256k1"
	evmtypes "github.com/tharsis/ethermint/x/evm/types"
)

func (suite *KeeperTestSuite) TestOnRecvTransfer() {
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
				suite.app.GetSubspace(evmtypes.ModuleName),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
			)
			suite.app.CronosKeeper = cronosKeeper

			tc.malleate()
			suite.app.CronosKeeper.OnRecvTransfer(suite.ctx, tc.coins, address.String())
			tc.postCheck()
		})
	}
}
