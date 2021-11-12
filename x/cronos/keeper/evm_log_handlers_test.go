package keeper_test

import (
	"errors"
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/app"
	"github.com/crypto-org-chain/cronos/x/cronos/keeper"
	cronosmodulekeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	keepertest "github.com/crypto-org-chain/cronos/x/cronos/keeper/mock"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	gravitykeeper "github.com/peggyjv/gravity-bridge/module/x/gravity/keeper"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/x/gravity/types"
)

func (suite *KeeperTestSuite) TestSendToAccountHandler() {
	contract := common.BigToAddress(big.NewInt(1))
	recipient := common.BigToAddress(big.NewInt(3))
	denom := "testdenom"
	var data []byte

	testCases := []struct {
		msg       string
		malleate  func()
		postcheck func()
		error     error
	}{
		{
			"nil data, expect success",
			func() {
				data = nil
			},
			func() {},
			nil,
		},
		{
			"not enough balance, expect fail",
			func() {
				input, err := keeper.SendToAccountEvent.Inputs.Pack(
					recipient,
					big.NewInt(100),
				)
				data = input
				suite.Require().NoError(err)
			},
			func() {},
			errors.New("contract 0x0000000000000000000000000000000000000001 is not connected to native token"),
		},
		{
			"success send to account",
			func() {
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, denom, contract)
				coin := sdk.NewCoin(denom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(coin, balance)

				input, err := keeper.SendToAccountEvent.Inputs.Pack(
					recipient,
					coin.Amount.BigInt(),
				)
				suite.Require().NoError(err)
				data = input
			},
			func() {
				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(sdk.NewCoin(denom, sdk.NewInt(0)), balance)
				balance = suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(recipient.Bytes()), denom)
				coin := sdk.NewCoin(denom, sdk.NewInt(100))
				suite.Require().Equal(coin, balance)
			},
			nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			handler := keeper.NewSendToAccountHandler(suite.app.BankKeeper, suite.app.CronosKeeper)
			tc.malleate()
			err := handler.Handle(suite.ctx, contract, data)
			if tc.error != nil {
				suite.Require().EqualError(err, tc.error.Error())
			} else {
				suite.Require().NoError(err)
				tc.postcheck()
			}
		})
	}
}

func (suite *KeeperTestSuite) TestSendToEthereumHandler() {
	suite.SetupTest()

	contract := common.BigToAddress(big.NewInt(1))
	recipient := common.BigToAddress(big.NewInt(3))
	invalidDenom := "testdenom"
	validDenom := "gravity0x0000000000000000000000000000000000000000"
	var data []byte

	testCases := []struct {
		msg       string
		malleate  func()
		postcheck func()
		error     error
	}{
		{
			"non gravity denom, expect fail",
			func() {
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, invalidDenom, contract)
				coin := sdk.NewCoin(invalidDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), invalidDenom)
				suite.Require().Equal(coin, balance)

				input, err := keeper.SendToEthereumEvent.Inputs.Pack(
					recipient,
					coin.Amount.BigInt(),
					big.NewInt(0),
				)
				data = input
			},
			func() {},
			errors.New("the native token associated with the contract 0x0000000000000000000000000000000000000001 is not a gravity voucher"),
		},
		{
			"non associated coin denom, expect fail",
			func() {
				coin := sdk.NewCoin(invalidDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), invalidDenom)
				suite.Require().Equal(coin, balance)

				input, err := keeper.SendToEthereumEvent.Inputs.Pack(
					recipient,
					coin.Amount.BigInt(),
					big.NewInt(0),
				)
				data = input
			},
			func() {},
			errors.New("contract 0x0000000000000000000000000000000000000001 is not connected to native token"),
		},
		{
			"success send to ethereum",
			func() {
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, validDenom, contract)
				coin := sdk.NewCoin(validDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), validDenom)
				suite.Require().Equal(coin, balance)

				input, err := keeper.SendToEthereumEvent.Inputs.Pack(
					recipient,
					coin.Amount.BigInt(),
					big.NewInt(0),
				)
				data = input
			},
			func() {
				// sender's balance deducted
				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), validDenom)
				suite.Require().Equal(sdk.NewCoin(validDenom, sdk.NewInt(0)), balance)
				// query unbatched SendToEthereum message exist
				rsp, err := suite.app.GravityKeeper.UnbatchedSendToEthereums(sdk.WrapSDKContext(suite.ctx), &gravitytypes.UnbatchedSendToEthereumsRequest{
					SenderAddress: sdk.AccAddress(contract.Bytes()).String(),
				})
				suite.Require().Equal(1, len(rsp.SendToEthereums))
				suite.Require().NoError(err)
			},
			nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest()
			handler := keeper.NewSendToEthereumHandler(
				gravitykeeper.NewMsgServerImpl(suite.app.GravityKeeper), suite.app.CronosKeeper)
			tc.malleate()
			err := handler.Handle(suite.ctx, contract, data)
			if tc.error != nil {
				suite.Require().EqualError(err, tc.error.Error())
			} else {
				suite.Require().NoError(err)
				tc.postcheck()
			}
		})
	}
}

func (suite *KeeperTestSuite) TestSendToIbcHandler() {
	contract := common.BigToAddress(big.NewInt(1))
	sender := common.BigToAddress(big.NewInt(2))
	invalidDenom := "testdenom"
	validDenom := CorrectIbcDenom
	var data []byte

	testCases := []struct {
		msg       string
		malleate  func()
		postcheck func()
		error     error
	}{
		{
			"non associated coin denom, expect fail",
			func() {
				coin := sdk.NewCoin(invalidDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), invalidDenom)
				suite.Require().Equal(coin, balance)

				input, err := keeper.SendToIbcEvent.Inputs.Pack(
					sender,
					"recipient",
					coin.Amount.BigInt(),
				)
				data = input
			},
			func() {},
			errors.New("contract 0x0000000000000000000000000000000000000001 is not connected to native token"),
		},
		{
			"non IBC denom, expect fail",
			func() {
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, invalidDenom, contract)
				coin := sdk.NewCoin(invalidDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), invalidDenom)
				suite.Require().Equal(coin, balance)

				input, err := keeper.SendToIbcEvent.Inputs.Pack(
					sender,
					"recipient",
					coin.Amount.BigInt(),
				)
				data = input
			},
			func() {},
			errors.New("the native token associated with the contract 0x0000000000000000000000000000000000000001 is not an ibc voucher"),
		},
		{
			"success send to ibc",
			func() {
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, validDenom, contract)
				coin := sdk.NewCoin(validDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), validDenom)
				suite.Require().Equal(coin, balance)

				input, err := keeper.SendToIbcEvent.Inputs.Pack(
					sender,
					"recipient",
					coin.Amount.BigInt(),
				)
				data = input
			},
			func() {},
			nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest()
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
			)
			handler := keeper.NewSendToIbcHandler(suite.app.BankKeeper, cronosKeeper)
			tc.malleate()
			err := handler.Handle(suite.ctx, contract, data)
			if tc.error != nil {
				suite.Require().EqualError(err, tc.error.Error())
			} else {
				suite.Require().NoError(err)
				tc.postcheck()
			}
		})
	}
}

func (suite *KeeperTestSuite) TestSendCroToIbcHandler() {
	contract := common.BigToAddress(big.NewInt(1))
	sender := common.BigToAddress(big.NewInt(2))
	var data []byte

	testCases := []struct {
		msg       string
		malleate  func()
		postcheck func()
		error     error
	}{
		{
			"not enough balance, fail",
			func() {
				coin := sdk.NewCoin(suite.evmParam.EvmDenom, sdk.NewInt(10000000000000))
				input, err := keeper.SendCroToIbcEvent.Inputs.Pack(
					sender,
					"recipient",
					coin.Amount.BigInt(),
				)
				suite.Require().NoError(err)
				data = input
			},
			func() {},
			errors.New("0aphoton is smaller than 10000000000000aphoton: insufficient funds"),
		},
		{
			"success send cro to ibc",
			func() {
				coin := sdk.NewCoin(suite.evmParam.EvmDenom, sdk.NewInt(1230000000500))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), suite.evmParam.EvmDenom)
				suite.Require().Equal(coin, balance)

				// Mint coin for the module
				suite.MintCoinsToModule(types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdk.NewInt(123))))
				input, err := keeper.SendToIbcEvent.Inputs.Pack(
					sender,
					"recipient",
					coin.Amount.BigInt(),
				)
				data = input
			},
			func() {
				// Verify balance post operation
				coin := sdk.NewCoin(types.IbcCroDenomDefaultValue, sdk.NewInt(0))
				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(types.ModuleName), types.IbcCroDenomDefaultValue)
				suite.Require().Equal(coin, balance)
				ibcCoin := sdk.NewCoin(types.IbcCroDenomDefaultValue, sdk.NewInt(123))
				// As we mock IBC module, we expect the token to be in user balance
				ibcBalance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(sender.Bytes()), types.IbcCroDenomDefaultValue)
				suite.Require().Equal(ibcCoin, ibcBalance)
				croCoin := sdk.NewCoin(suite.evmParam.EvmDenom, sdk.NewInt(500))
				croBalance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(sender.Bytes()), suite.evmParam.EvmDenom)
				suite.Require().Equal(croCoin, croBalance)
			},
			nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest()
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
			)
			handler := keeper.NewSendCroToIbcHandler(suite.app.BankKeeper, cronosKeeper)
			tc.malleate()
			err := handler.Handle(suite.ctx, contract, data)
			if tc.error != nil {
				suite.Require().EqualError(err, tc.error.Error())
			} else {
				suite.Require().NoError(err)
				tc.postcheck()
			}
		})
	}
}
