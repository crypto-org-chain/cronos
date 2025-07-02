package keeper_test

import (
	"errors"
	"fmt"
	"math/big"

	cronosmodulekeeper "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper"
	keepertest "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper/mock"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

const (
	CorrectIbcDenom    = "ibc/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	CorrectCronosDenom = "cronos0xc1b37f2abdb778f540fa5db8e1fd2eadfc9a05ed"
)

func (suite *KeeperTestSuite) TestConvertVouchersToEvmCoins() {
	privKey, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)
	address := sdk.AccAddress(privKey.PubKey().Address())

	testCases := []struct {
		name          string
		from          string
		coin          sdk.Coins
		malleate      func()
		expectedError error
		postCheck     func()
	}{
		{
			"Wrong from address",
			"test",
			sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(1))),
			func() {},
			errors.New("decoding bech32 failed: invalid bech32 string length 4"),
			func() {},
		},
		{
			"Empty address",
			"",
			sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(1))),
			func() {},
			errors.New("empty address string is not allowed"),
			func() {},
		},
		{
			"Correct address with non supported coin denom",
			address.String(),
			sdk.NewCoins(sdk.NewCoin("fake", sdkmath.NewInt(1))),
			func() {},
			errors.New("coin fake is not supported for conversion"),
			func() {},
		},
		{
			"Correct address with not enough IBC CRO token",
			address.String(),
			sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123))),
			func() {},
			errors.New("spendable balance 0ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865 is smaller than 123ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865: insufficient funds"),
			func() {},
		},
		{
			"Correct address with enough IBC CRO token : Should receive CRO tokens",
			address.String(),
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
			nil,
			func() {
				// Verify balance IBC coin post operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdkmath.NewInt(0), ibcCroCoin.Amount)
				// Verify balance EVM coin post operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdkmath.NewInt(1230000000000), evmCoin.Amount)
			},
		},
		{
			"Correct address with not enough IBC token",
			address.String(),
			sdk.NewCoins(sdk.NewCoin(CorrectIbcDenom, sdkmath.NewInt(1))),
			func() {},
			fmt.Errorf("spendable balance 0%s is smaller than 1%s: insufficient funds", CorrectIbcDenom, CorrectIbcDenom),
			func() {},
		},
		{
			"Correct address with IBC token : Should receive CRC20 tokens",
			address.String(),
			sdk.NewCoins(sdk.NewCoin(CorrectIbcDenom, sdkmath.NewInt(123))),
			func() {
				err := suite.MintCoins(address, sdk.NewCoins(sdk.NewCoin(CorrectIbcDenom, sdkmath.NewInt(123))))
				suite.Require().NoError(err)
				// Verify balance IBC coin pre operation
				ibcCroCoin := suite.GetBalance(address, CorrectIbcDenom)
				suite.Require().Equal(sdkmath.NewInt(123), ibcCroCoin.Amount)
			},
			nil,
			func() {
				// Verify balance IBC coin post operation
				ibcCroCoin := suite.GetBalance(address, CorrectIbcDenom)
				suite.Require().Equal(sdkmath.NewInt(0), ibcCroCoin.Amount)
				// Verify CRC20 balance post operation
				contract, found := suite.app.CronosKeeper.GetContractByDenom(suite.ctx, CorrectIbcDenom)
				suite.Require().True(found)
				ret, err := suite.app.CronosKeeper.CallModuleCRC21(suite.ctx, contract, "balanceOf", common.BytesToAddress(address.Bytes()))
				suite.Require().NoError(err)
				suite.Require().Equal(big.NewInt(123), big.NewInt(0).SetBytes(ret))
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			tc.malleate()
			err := suite.app.CronosKeeper.ConvertVouchersToEvmCoins(suite.ctx, tc.from, tc.coin)
			if tc.expectedError != nil {
				suite.Require().EqualError(err, tc.expectedError.Error())
			} else {
				suite.Require().NoError(err)
				tc.postCheck()
			}
		})
	}
}

func (suite *KeeperTestSuite) TestIbcTransferCoins() {
	privKey, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)
	address := sdk.AccAddress(privKey.PubKey().Address())

	testCases := []struct {
		name          string
		from          string
		to            string
		coin          sdk.Coins
		channelId     string
		malleate      func()
		expectedError error
		postCheck     func()
	}{
		{
			"Wrong from address",
			"test",
			"to",
			sdk.NewCoins(sdk.NewCoin(suite.evmParam.EvmDenom, sdkmath.NewInt(1))),
			"channel-0",
			func() {},
			errors.New("decoding bech32 failed: invalid bech32 string length 4"),
			func() {},
		},
		{
			"Empty address",
			"",
			"to",
			sdk.NewCoins(sdk.NewCoin(suite.evmParam.EvmDenom, sdkmath.NewInt(1))),
			"channel-0",
			func() {},
			errors.New("empty address string is not allowed"),
			func() {},
		},
		{
			"Correct address with non supported coin denom",
			address.String(),
			"to",
			sdk.NewCoins(sdk.NewCoin("ibc/BAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", sdkmath.NewInt(1))),
			"channel-0",
			func() {},
			errors.New("coin ibc/BAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA is not supported"),
			func() {},
		},
		{
			"Correct address with incorrect coin denom",
			address.String(),
			"to",
			sdk.NewCoins(sdk.NewCoin("fake", sdkmath.NewInt(1))),
			"channel-0",
			func() {},
			errors.New("the coin fake is neither an ibc voucher or a cronos token"),
			func() {},
		},
		{
			"Correct address with too small amount EVM token",
			address.String(),
			"to",
			sdk.NewCoins(sdk.NewCoin(suite.evmParam.EvmDenom, sdkmath.NewInt(123))),
			"channel-0",
			func() {},
			nil,
			func() {},
		},
		{
			"Correct address with not enough EVM token",
			address.String(),
			"to",
			sdk.NewCoins(sdk.NewCoin(suite.evmParam.EvmDenom, sdkmath.NewInt(1230000000000))),
			"channel-0",
			func() {},
			errors.New("spendable balance 0aphoton is smaller than 1230000000000aphoton: insufficient funds"),
			func() {},
		},
		{
			"Correct address with enough EVM token : Should receive IBC CRO token",
			address.String(),
			"to",
			sdk.NewCoins(sdk.NewCoin(suite.evmParam.EvmDenom, sdkmath.NewInt(1230000000000))),
			"channel-0",
			func() {
				// Mint Coin to user and module
				err := suite.MintCoins(address, sdk.NewCoins(sdk.NewCoin(suite.evmParam.EvmDenom, sdkmath.NewInt(1230000000000))))
				suite.Require().NoError(err)
				err = suite.MintCoinsToModule(types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123))))
				suite.Require().NoError(err)
				// Verify balance IBC coin pre operation
				ibcCroCoin := suite.GetBalance(address, types.IbcCroDenomDefaultValue)
				suite.Require().Equal(sdkmath.NewInt(0), ibcCroCoin.Amount)
				// Verify balance EVM coin pre operation
				evmCoin := suite.GetBalance(address, suite.evmParam.EvmDenom)
				suite.Require().Equal(sdkmath.NewInt(1230000000000), evmCoin.Amount)
			},
			nil,
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
			"Correct address with non correct IBC token denom",
			address.String(),
			"to",
			sdk.NewCoins(sdk.NewCoin("incorrect", sdkmath.NewInt(123))),
			"channel-0",
			func() {
				// Add support for the IBC token
				suite.app.CronosKeeper.SetAutoContractForDenom(suite.ctx, "incorrect", common.HexToAddress("0x11"))
			},
			errors.New("the coin incorrect is neither an ibc voucher or a cronos token"),
			func() {
			},
		},
		{
			"Correct address with correct IBC token denom",
			address.String(),
			"to",
			sdk.NewCoins(sdk.NewCoin(CorrectIbcDenom, sdkmath.NewInt(123))),
			"channel-0",
			func() {
				// Mint IBC token for user
				err := suite.MintCoins(address, sdk.NewCoins(sdk.NewCoin(CorrectIbcDenom, sdkmath.NewInt(123))))
				suite.Require().NoError(err)
				// Add support for the IBC token
				suite.app.CronosKeeper.SetAutoContractForDenom(suite.ctx, CorrectIbcDenom, common.HexToAddress("0x11"))
			},
			nil,
			func() {
			},
		},
		{
			"Correct address with incorrect IBC token denom",
			address.String(),
			"to",
			sdk.NewCoins(sdk.NewCoin(CorrectCronosDenom, sdkmath.NewInt(123))),
			"aaa",
			func() {
				// Mint IBC token for user
				err := suite.MintCoins(address, sdk.NewCoins(sdk.NewCoin(CorrectCronosDenom, sdkmath.NewInt(123))))
				suite.Require().NoError(err)
				// Add support for the IBC token
				suite.app.CronosKeeper.SetAutoContractForDenom(suite.ctx, CorrectCronosDenom, common.HexToAddress("0x11"))
			},
			errors.New("invalid channel id for ibc transfer of source token"),
			func() {
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
			err := suite.app.CronosKeeper.IbcTransferCoins(suite.ctx, tc.from, tc.to, tc.coin, tc.channelId)
			if tc.expectedError != nil {
				suite.Require().EqualError(err, tc.expectedError.Error())
			} else {
				suite.Require().NoError(err)
				tc.postCheck()
			}
		})
	}
}
