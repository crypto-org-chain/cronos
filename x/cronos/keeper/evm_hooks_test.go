package keeper_test

import (
	"fmt"
	"math/big"

	handlers "github.com/crypto-org-chain/cronos/x/cronos/keeper/evmhandlers"

	gravitytypes "github.com/peggyjv/gravity-bridge/module/v2/x/gravity/types"

	"github.com/crypto-org-chain/cronos/app"
	keepertest "github.com/crypto-org-chain/cronos/x/cronos/keeper/mock"
	"github.com/crypto-org-chain/cronos/x/cronos/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	cronosmodulekeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func (suite *KeeperTestSuite) TestEvmHooks() {
	suite.SetupTest()

	contract := common.BigToAddress(big.NewInt(1))
	recipient := common.BigToAddress(big.NewInt(3))
	sender := common.BigToAddress(big.NewInt(4))

	testCases := []struct {
		msg      string
		malleate func()
	}{
		{
			"invalid log data, but still success",
			func() {
				logs := []*ethtypes.Log{
					{
						Address: contract,
						Topics:  []common.Hash{handlers.SendToAccountEvent.ID},
					},
				}
				receipt := &ethtypes.Receipt{
					Logs: logs,
				}
				err := suite.app.EvmKeeper.PostTxProcessing(suite.ctx, nil, receipt)
				suite.Require().NoError(err)
			},
		},
		{
			"not enough balance, expect fail",
			func() {
				data, err := handlers.SendToAccountEvent.Inputs.Pack(
					recipient,
					big.NewInt(100),
				)
				suite.Require().NoError(err)
				logs := []*ethtypes.Log{
					{
						Address: contract,
						Topics:  []common.Hash{handlers.SendToAccountEvent.ID},
						Data:    data,
					},
				}
				receipt := &ethtypes.Receipt{
					Logs: logs,
				}
				err = suite.app.EvmKeeper.PostTxProcessing(suite.ctx, nil, receipt)
				suite.Require().Error(err)
			},
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

				data, err := handlers.SendToAccountEvent.Inputs.Pack(
					recipient,
					coin.Amount.BigInt(),
				)
				suite.Require().NoError(err)
				logs := []*ethtypes.Log{
					{
						Address: contract,
						Topics:  []common.Hash{handlers.SendToAccountEvent.ID},
						Data:    data,
					},
				}
				receipt := &ethtypes.Receipt{
					Logs: logs,
				}
				err = suite.app.EvmKeeper.PostTxProcessing(suite.ctx, nil, receipt)
				suite.Require().NoError(err)

				balance = suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(sdk.NewCoin(denom, sdk.NewInt(0)), balance)
				balance = suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(recipient.Bytes()), denom)
				suite.Require().Equal(coin, balance)
			},
		},
		{
			"failed send to ethereum, invalid gravity denom",
			func() {
				suite.SetupTest()

				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, denom, contract)
				coin := sdk.NewCoin(denom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(sender.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(sender.Bytes()), denom)
				suite.Require().Equal(coin, balance)

				data, err := handlers.SendToChainEvent.Inputs.Pack(
					sender,
					recipient,
					coin.Amount.BigInt(),
					big.NewInt(0),
					big.NewInt(1),
				)
				suite.Require().NoError(err)
				logs := []*ethtypes.Log{
					{
						Address: contract,
						Topics:  []common.Hash{handlers.SendToChainEvent.ID},
						Data:    data,
					},
				}
				receipt := &ethtypes.Receipt{
					Logs: logs,
				}
				err = suite.app.EvmKeeper.PostTxProcessing(suite.ctx, nil, receipt)
				// should fail, because of not gravity denom name
				suite.Require().Error(err)
			},
		},
		{
			"success send to chain",
			func() {
				suite.SetupTest()
				denom := denomGravity

				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, denom, contract)
				coin := sdk.NewCoin(denom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(coin, balance)

				data, err := handlers.SendToChainEvent.Inputs.Pack(
					sender,
					recipient,
					coin.Amount.BigInt(),
					big.NewInt(0),
					big.NewInt(1),
				)
				suite.Require().NoError(err)
				logs := []*ethtypes.Log{
					{
						Address: contract,
						Topics:  []common.Hash{handlers.SendToChainEvent.ID},
						Data:    data,
					},
				}
				receipt := &ethtypes.Receipt{
					Logs: logs,
				}
				err = suite.app.EvmKeeper.PostTxProcessing(suite.ctx, nil, receipt)
				suite.Require().NoError(err)

				// contract's balance deducted
				balance = suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(sdk.NewCoin(denom, sdk.NewInt(0)), balance)
				// query unbatched SendToEthereum message exist
				rsp, _ := suite.app.GravityKeeper.UnbatchedSendToEthereums(sdk.WrapSDKContext(suite.ctx), &gravitytypes.UnbatchedSendToEthereumsRequest{
					SenderAddress: sdk.AccAddress(sender.Bytes()).String(),
				})
				suite.Require().Equal(1, len(rsp.SendToEthereums))
			},
		},
		{
			"failed send to ibc, invalid ibc denom",
			func() {
				suite.SetupTest()
				// Create Cronos Keeper with mock transfer keeper
				cronosKeeper := *cronosmodulekeeper.NewKeeper(
					app.MakeEncodingConfig().Codec,
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

				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, denom, contract)
				coin := sdk.NewCoin(denom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(coin, balance)

				data, err := handlers.SendToIbcEvent.Inputs.Pack(
					sender,
					"recipient",
					coin.Amount.BigInt(),
				)
				suite.Require().NoError(err)
				logs := []*ethtypes.Log{
					{
						Address: contract,
						Topics:  []common.Hash{handlers.SendToIbcEvent.ID},
						Data:    data,
					},
				}
				receipt := &ethtypes.Receipt{
					Logs: logs,
				}
				err = suite.app.EvmKeeper.PostTxProcessing(suite.ctx, nil, receipt)
				// should fail, because of not ibc denom name
				suite.Require().Error(err)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			tc.malleate()
		})
	}
}
