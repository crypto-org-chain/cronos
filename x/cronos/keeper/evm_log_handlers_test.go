package keeper_test

import (
	"errors"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/app"
	"github.com/crypto-org-chain/cronos/x/cronos/keeper"
	cronosmodulekeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	keepertest "github.com/crypto-org-chain/cronos/x/cronos/keeper/mock"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	gravitykeeper "github.com/peggyjv/gravity-bridge/module/x/gravity/keeper"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/x/gravity/types"
	"math/big"
)

func (suite *KeeperTestSuite) TestNativeTransferHandler() {
	suite.SetupTest()

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

func (suite *KeeperTestSuite) TestEthereumTransferHandler() {
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

func (suite *KeeperTestSuite) TestIbcTransferHandler() {
	suite.SetupTest()

	contract := common.BigToAddress(big.NewInt(1))
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
			"non IBC denom, expect fail",
			func() {
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, invalidDenom, contract)
				coin := sdk.NewCoin(invalidDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), invalidDenom)
				suite.Require().Equal(coin, balance)

				input, err := keeper.SendToIbcEvent.Inputs.Pack(
					"recipient",
					coin.Amount.BigInt(),
				)
				data = input
			},
			func() {},
			errors.New("contract 0x0000000000000000000000000000000000000001 is not connected to native token"),
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
			handler := keeper.NewSendToIbcHandler(cronosKeeper)
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
