package keeper_test

import (
	"errors"
	"fmt"
	"math/big"

	cronosmodulekeeper "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper"
	evmhandlers "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper/evmhandlers"
	keepertest "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper/mock"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

func (suite *KeeperTestSuite) TestSendToAccountHandler() {
	contract := common.BigToAddress(big.NewInt(1))
	recipient := common.BigToAddress(big.NewInt(3))
	var data []byte
	var topics []common.Hash

	testCases := []struct {
		msg       string
		malleate  func()
		postcheck func()
		error     error
	}{
		{
			"nil data, expect success",
			func() {
				topics = []common.Hash{
					evmhandlers.SendToAccountEvent.ID,
				}
				data = nil
			},
			func() {},
			nil,
		},
		{
			"not enough balance, expect fail",
			func() {
				topics = []common.Hash{
					evmhandlers.SendToAccountEvent.ID,
				}
				input, err := evmhandlers.SendToAccountEvent.Inputs.NonIndexed().Pack(
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
				err := suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, denom, contract)
				suite.Require().NoError(err)
				coin := sdk.NewCoin(denom, sdkmath.NewInt(100))
				err = suite.MintCoins(contract.Bytes(), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, contract.Bytes(), denom)
				suite.Require().Equal(coin, balance)

				topics = []common.Hash{
					evmhandlers.SendToAccountEvent.ID,
				}
				input, err := evmhandlers.SendToAccountEvent.Inputs.NonIndexed().Pack(
					recipient,
					coin.Amount.BigInt(),
				)
				suite.Require().NoError(err)
				data = input
			},
			func() {
				balance := suite.app.BankKeeper.GetBalance(suite.ctx, contract.Bytes(), denom)
				suite.Require().Equal(sdk.NewCoin(denom, sdkmath.NewInt(0)), balance)
				balance = suite.app.BankKeeper.GetBalance(suite.ctx, recipient.Bytes(), denom)
				coin := sdk.NewCoin(denom, sdkmath.NewInt(100))
				suite.Require().Equal(coin, balance)
			},
			nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			handler := evmhandlers.NewSendToAccountHandler(suite.app.BankKeeper, suite.app.CronosKeeper)
			tc.malleate()
			err := handler.Handle(suite.ctx, contract, topics, data, func(contractAddress common.Address, logSig common.Hash, logData []byte) {})
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
	invalidDenom := denom
	validDenom := CorrectIbcDenom
	var data []byte
	var topics []common.Hash

	testCases := []struct {
		msg       string
		malleate  func()
		postcheck func()
		error     error
	}{
		{
			"non associated coin denom, expect fail",
			func() {
				coin := sdk.NewCoin(invalidDenom, sdkmath.NewInt(100))
				err := suite.MintCoins(contract.Bytes(), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, contract.Bytes(), invalidDenom)
				suite.Require().Equal(coin, balance)

				topics = []common.Hash{
					evmhandlers.SendToIbcEvent.ID,
				}
				input, _ := evmhandlers.SendToIbcEvent.Inputs.NonIndexed().Pack(
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
				err := suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, invalidDenom, contract)
				suite.Require().NoError(err)
				coin := sdk.NewCoin(invalidDenom, sdkmath.NewInt(100))
				err = suite.MintCoins(contract.Bytes(), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, contract.Bytes(), invalidDenom)
				suite.Require().Equal(coin, balance)

				topics = []common.Hash{
					evmhandlers.SendToIbcEvent.ID,
				}
				input, _ := evmhandlers.SendToIbcEvent.Inputs.NonIndexed().Pack(
					sender,
					"recipient",
					coin.Amount.BigInt(),
				)
				data = input
			},
			func() {},
			errors.New("the native token associated with the contract 0x0000000000000000000000000000000000000001 is neither an ibc voucher or a cronos token"),
		},
		{
			"success send to ibc",
			func() {
				err := suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, validDenom, contract)
				suite.Require().NoError(err)
				coin := sdk.NewCoin(validDenom, sdkmath.NewInt(100))
				err = suite.MintCoins(contract.Bytes(), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, contract.Bytes(), validDenom)
				suite.Require().Equal(coin, balance)

				topics = []common.Hash{
					evmhandlers.SendToIbcEvent.ID,
				}
				input, _ := evmhandlers.SendToIbcEvent.Inputs.NonIndexed().Pack(
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
				suite.app.EncodingConfig().Codec,
				suite.app.GetKey(types.StoreKey),
				suite.app.GetKey(types.MemStoreKey),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
				suite.app.EvmKeeper,
				suite.app.AccountKeeper,
				authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			)
			handler := evmhandlers.NewSendToIbcHandler(suite.app.BankKeeper, cronosKeeper)
			tc.malleate()
			err := handler.Handle(suite.ctx, contract, topics, data, func(contractAddress common.Address, logSig common.Hash, logData []byte) {})
			if tc.error != nil {
				suite.Require().EqualError(err, tc.error.Error())
			} else {
				suite.Require().NoError(err)
				tc.postcheck()
			}
		})
	}
}

func (suite *KeeperTestSuite) TestSendToIbcV2Handler() {
	contract := common.BigToAddress(big.NewInt(1))
	sender := common.BigToAddress(big.NewInt(2))
	recipient := "recipient"
	invalidDenom := denom
	validDenom := CorrectIbcDenom
	var data []byte
	var topics []common.Hash

	testCases := []struct {
		msg       string
		malleate  func()
		postcheck func()
		error     error
	}{
		{
			"non associated coin denom, expect fail",
			func() {
				coin := sdk.NewCoin(invalidDenom, sdkmath.NewInt(100))
				err := suite.MintCoins(contract.Bytes(), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, contract.Bytes(), invalidDenom)
				suite.Require().Equal(coin, balance)

				topics = []common.Hash{
					evmhandlers.SendToIbcEvent.ID,
					sender.Hash(),
					common.BytesToHash(big.NewInt(0).Bytes()),
				}
				input, _ := evmhandlers.SendToIbcEventV2.Inputs.NonIndexed().Pack(
					recipient,
					coin.Amount.BigInt(),
					[]byte{},
				)
				data = input
			},
			func() {},
			errors.New("contract 0x0000000000000000000000000000000000000001 is not connected to native token"),
		},
		{
			"non IBC denom, expect fail",
			func() {
				err := suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, invalidDenom, contract)
				suite.Require().NoError(err)
				coin := sdk.NewCoin(invalidDenom, sdkmath.NewInt(100))
				err = suite.MintCoins(contract.Bytes(), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, contract.Bytes(), invalidDenom)
				suite.Require().Equal(coin, balance)

				topics = []common.Hash{
					evmhandlers.SendToIbcEvent.ID,
					sender.Hash(),
					common.BytesToHash(big.NewInt(0).Bytes()),
				}
				input, _ := evmhandlers.SendToIbcEventV2.Inputs.NonIndexed().Pack(
					recipient,
					coin.Amount.BigInt(),
					[]byte{},
				)
				data = input
			},
			func() {},
			errors.New("the native token associated with the contract 0x0000000000000000000000000000000000000001 is neither an ibc voucher or a cronos token"),
		},
		{
			"success send to ibc",
			func() {
				err := suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, validDenom, contract)
				suite.Require().NoError(err)
				coin := sdk.NewCoin(validDenom, sdkmath.NewInt(100))
				err = suite.MintCoins(contract.Bytes(), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, contract.Bytes(), validDenom)
				suite.Require().Equal(coin, balance)

				topics = []common.Hash{
					evmhandlers.SendToIbcEvent.ID,
					sender.Hash(),
					common.BytesToHash(big.NewInt(0).Bytes()),
				}
				input, _ := evmhandlers.SendToIbcEventV2.Inputs.NonIndexed().Pack(
					recipient,
					coin.Amount.BigInt(),
					[]byte{},
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
				suite.app.EncodingConfig().Codec,
				suite.app.GetKey(types.StoreKey),
				suite.app.GetKey(types.MemStoreKey),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
				suite.app.EvmKeeper,
				suite.app.AccountKeeper,
				authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			)
			handler := evmhandlers.NewSendToIbcV2Handler(suite.app.BankKeeper, cronosKeeper)
			tc.malleate()
			err := handler.Handle(suite.ctx, contract, topics, data, func(contractAddress common.Address, logSig common.Hash, logData []byte) {})
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
	var topics []common.Hash

	testCases := []struct {
		msg       string
		malleate  func()
		postcheck func()
		error     error
	}{
		{
			"not enough balance, fail",
			func() {
				coin := sdk.NewCoin(suite.evmParam.EvmDenom, sdkmath.NewInt(10000000000000))
				topics = []common.Hash{
					evmhandlers.SendCroToIbcEvent.ID,
				}
				input, err := evmhandlers.SendCroToIbcEvent.Inputs.NonIndexed().Pack(
					sender,
					"recipient",
					coin.Amount.BigInt(),
				)
				suite.Require().NoError(err)
				data = input
			},
			func() {},
			errors.New("spendable balance 0aphoton is smaller than 10000000000000aphoton: insufficient funds"),
		},
		{
			"success send cro to ibc",
			func() {
				coin := sdk.NewCoin(suite.evmParam.EvmDenom, sdkmath.NewInt(1230000000500))
				err := suite.MintCoins(contract.Bytes(), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, contract.Bytes(), suite.evmParam.EvmDenom)
				suite.Require().Equal(coin, balance)

				// Mint coin for the module
				err = suite.MintCoinsToModule(types.ModuleName, sdk.NewCoins(sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123))))
				suite.Require().NoError(err)
				topics = []common.Hash{
					evmhandlers.SendCroToIbcEvent.ID,
				}
				input, _ := evmhandlers.SendToIbcEvent.Inputs.NonIndexed().Pack(
					sender,
					"recipient",
					coin.Amount.BigInt(),
				)
				data = input
			},
			func() {
				// Verify balance post operation
				coin := sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(0))
				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(types.ModuleName), types.IbcCroDenomDefaultValue)
				suite.Require().Equal(coin, balance)
				ibcCoin := sdk.NewCoin(types.IbcCroDenomDefaultValue, sdkmath.NewInt(123))
				// As we mock IBC module, we expect the token to be in user balance
				ibcBalance := suite.app.BankKeeper.GetBalance(suite.ctx, sender.Bytes(), types.IbcCroDenomDefaultValue)
				suite.Require().Equal(ibcCoin, ibcBalance)
				croCoin := sdk.NewCoin(suite.evmParam.EvmDenom, sdkmath.NewInt(500))
				croBalance := suite.app.BankKeeper.GetBalance(suite.ctx, sender.Bytes(), suite.evmParam.EvmDenom)
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
				suite.app.EncodingConfig().Codec,
				suite.app.GetKey(types.StoreKey),
				suite.app.GetKey(types.MemStoreKey),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
				suite.app.EvmKeeper,
				suite.app.AccountKeeper,
				authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			)
			handler := evmhandlers.NewSendCroToIbcHandler(suite.app.BankKeeper, cronosKeeper)
			tc.malleate()
			err := handler.Handle(suite.ctx, contract, topics, data, func(contractAddress common.Address, logSig common.Hash, logData []byte) {})
			if tc.error != nil {
				suite.Require().EqualError(err, tc.error.Error())
			} else {
				suite.Require().NoError(err)
				tc.postcheck()
			}
		})
	}
}
