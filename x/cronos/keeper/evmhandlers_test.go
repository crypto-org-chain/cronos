package keeper_test

import (
	"errors"
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/crypto-org-chain/cronos/v2/app"
	cronosmodulekeeper "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper"
	evmhandlers "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper/evmhandlers"
	keepertest "github.com/crypto-org-chain/cronos/v2/x/cronos/keeper/mock"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	gravitykeeper "github.com/peggyjv/gravity-bridge/module/v2/x/gravity/keeper"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/v2/x/gravity/types"
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
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, denom, contract)
				coin := sdk.NewCoin(denom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
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

func (suite *KeeperTestSuite) TestSendToEvmChainHandler() {
	suite.SetupTest()

	contract := common.BigToAddress(big.NewInt(1))
	sender := common.BigToAddress(big.NewInt(2))
	recipient := common.BigToAddress(big.NewInt(3))
	invalidDenom := denom
	validDenom := denomGravity
	var data []byte
	var topics []common.Hash

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

				topics = []common.Hash{
					evmhandlers.SendToEvmChainEvent.ID,
					sender.Hash(),
					recipient.Hash(),
					common.BytesToHash(big.NewInt(1).Bytes()),
				}

				input, _ := evmhandlers.SendToEvmChainEvent.Inputs.NonIndexed().Pack(
					coin.Amount.BigInt(),
					big.NewInt(0),
					[]byte{},
				)
				data = input
			},
			func() {},
			errors.New("the native token associated with the contract 0x0000000000000000000000000000000000000001 is neither a gravity voucher or a cronos token"),
		},
		{
			"non supported network id",
			func() {
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, validDenom, contract)
				coin := sdk.NewCoin(validDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), validDenom)
				suite.Require().Equal(coin, balance)

				topics = []common.Hash{
					evmhandlers.SendToEvmChainEvent.ID,
					sender.Hash(),
					recipient.Hash(),
					common.BytesToHash(big.NewInt(100).Bytes()),
				}

				input, _ := evmhandlers.SendToEvmChainEvent.Inputs.NonIndexed().Pack(
					coin.Amount.BigInt(),
					big.NewInt(0),
					[]byte{},
				)
				data = input
			},
			func() {},
			errors.New("only ethereum network is supported"),
		},
		{
			"non associated coin denom, expect fail",
			func() {
				coin := sdk.NewCoin(invalidDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), invalidDenom)
				suite.Require().Equal(coin, balance)

				topics = []common.Hash{
					evmhandlers.SendToEvmChainEvent.ID,
					sender.Hash(),
					recipient.Hash(),
					common.BytesToHash(big.NewInt(1).Bytes()),
				}

				input, _ := evmhandlers.SendToEvmChainEvent.Inputs.NonIndexed().Pack(
					coin.Amount.BigInt(),
					big.NewInt(0),
					[]byte{},
				)
				data = input
			},
			func() {},
			errors.New("contract 0x0000000000000000000000000000000000000001 is not connected to native token"),
		},
		{
			"success send to chain",
			func() {
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, validDenom, contract)
				coin := sdk.NewCoin(validDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), validDenom)
				suite.Require().Equal(coin, balance)

				topics = []common.Hash{
					evmhandlers.SendToEvmChainEvent.ID,
					sender.Hash(),
					recipient.Hash(),
					common.BytesToHash(big.NewInt(1).Bytes()),
				}

				input, _ := evmhandlers.SendToEvmChainEvent.Inputs.NonIndexed().Pack(
					coin.Amount.BigInt(),
					big.NewInt(0),
					[]byte{},
				)
				data = input
			},
			func() {
				// sender's balance deducted
				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), validDenom)
				suite.Require().Equal(sdk.NewCoin(validDenom, sdk.NewInt(0)), balance)
				// query unbatched SendToEthereum message exist
				rsp, err := suite.app.GravityKeeper.UnbatchedSendToEthereums(sdk.WrapSDKContext(suite.ctx), &gravitytypes.UnbatchedSendToEthereumsRequest{
					SenderAddress: sdk.AccAddress(sender.Bytes()).String(),
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
			handler := evmhandlers.NewSendToEvmChainHandler(
				gravitykeeper.NewMsgServerImpl(suite.app.GravityKeeper),
				suite.app.BankKeeper, suite.app.CronosKeeper)
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
				coin := sdk.NewCoin(invalidDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), invalidDenom)
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
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, invalidDenom, contract)
				coin := sdk.NewCoin(invalidDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), invalidDenom)
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
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, validDenom, contract)
				coin := sdk.NewCoin(validDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), validDenom)
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
				app.MakeEncodingConfig().Codec,
				suite.app.GetKey(types.StoreKey),
				suite.app.GetKey(types.MemStoreKey),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
				suite.app.GravityKeeper,
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
				coin := sdk.NewCoin(invalidDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), invalidDenom)
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
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, invalidDenom, contract)
				coin := sdk.NewCoin(invalidDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), invalidDenom)
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
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, validDenom, contract)
				coin := sdk.NewCoin(validDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), validDenom)
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
				app.MakeEncodingConfig().Codec,
				suite.app.GetKey(types.StoreKey),
				suite.app.GetKey(types.MemStoreKey),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
				suite.app.GravityKeeper,
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
				coin := sdk.NewCoin(suite.evmParam.EvmDenom, sdk.NewInt(10000000000000))
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
			errors.New("spendable balance  is smaller than 10000000000000aphoton: insufficient funds"),
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
				app.MakeEncodingConfig().Codec,
				suite.app.GetKey(types.StoreKey),
				suite.app.GetKey(types.MemStoreKey),
				suite.app.BankKeeper,
				keepertest.IbcKeeperMock{},
				suite.app.GravityKeeper,
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

func (suite *KeeperTestSuite) TestCancelSendToEvmChainHandler() {
	suite.SetupTest()

	contract := common.BigToAddress(big.NewInt(1))
	sender := common.BigToAddress(big.NewInt(2))
	validDenom := denomGravity
	var data []byte
	var topics []common.Hash

	testCases := []struct {
		msg       string
		malleate  func()
		postcheck func()
		error     error
	}{
		{
			"id not found",
			func() {
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, validDenom, contract)
				coin := sdk.NewCoin(validDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(sender.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(sender.Bytes()), validDenom)
				suite.Require().Equal(coin, balance)

				topics = []common.Hash{
					evmhandlers.CancelSendToEvmChainEvent.ID,
					sender.Hash(),
				}
				input, _ := evmhandlers.CancelSendToEvmChainEvent.Inputs.NonIndexed().Pack(
					big.NewInt(1),
				)
				data = input
			},
			func() {},
			errors.New("id not found or the transaction is already included in a batch"),
		},
		{
			"success cancel send to chain",
			func() {
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, validDenom, contract)
				coin := sdk.NewCoin(validDenom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(sender.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)
				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(sender.Bytes()), validDenom)
				suite.Require().Equal(coin, balance)

				// First add a SendToChain transaction
				gravityMsgServer := gravitykeeper.NewMsgServerImpl(suite.app.GravityKeeper)
				msg := gravitytypes.MsgSendToEthereum{
					Sender:            sdk.AccAddress(sender.Bytes()).String(),
					EthereumRecipient: "0x000000000000000000000000000000000000dEaD",
					Amount:            sdk.NewCoin(validDenom, sdk.NewInt(99)),
					BridgeFee:         sdk.NewCoin(validDenom, sdk.NewInt(1)),
				}
				resp, err := gravityMsgServer.SendToEthereum(sdk.WrapSDKContext(suite.ctx), &msg)
				suite.Require().NoError(err)
				// check sender's balance deducted
				balance = suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(sender.Bytes()), validDenom)
				suite.Require().Equal(sdk.NewCoin(validDenom, sdk.NewInt(0)), balance)
				// check contract's balance empty
				balance = suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), validDenom)
				suite.Require().Equal(sdk.NewCoin(validDenom, sdk.NewInt(0)), balance)

				// Then cancel the SendToEvmChain transaction
				topics = []common.Hash{
					evmhandlers.CancelSendToEvmChainEvent.ID,
					sender.Hash(),
				}
				input, _ := evmhandlers.CancelSendToEvmChainEvent.Inputs.NonIndexed().Pack(
					big.NewInt(int64(resp.Id)),
				)
				data = input
			},
			func() {
				// sender's balance should be refunded and then send to the contract address
				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), validDenom)
				suite.Require().Equal(sdk.NewCoin(validDenom, sdk.NewInt(100)), balance)
				// query unbatched SendToChain message does not exist
				rsp, err := suite.app.GravityKeeper.UnbatchedSendToEthereums(sdk.WrapSDKContext(suite.ctx), &gravitytypes.UnbatchedSendToEthereumsRequest{
					SenderAddress: sdk.AccAddress(sender.Bytes()).String(),
				})
				suite.Require().Equal(0, len(rsp.SendToEthereums))
				suite.Require().NoError(err)
			},
			nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			suite.SetupTest()
			handler := evmhandlers.NewCancelSendToEvmChainHandler(
				gravitykeeper.NewMsgServerImpl(suite.app.GravityKeeper),
				suite.app.CronosKeeper, suite.app.GravityKeeper)
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
