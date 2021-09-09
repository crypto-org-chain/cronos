package keeper_test

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gravitytypes "github.com/peggyjv/gravity-bridge/module/x/gravity/types"

	"github.com/crypto-org-chain/cronos/x/cronos/keeper"
)

func (suite *KeeperTestSuite) TestEvmHooks() {
	suite.SetupTest()

	contract := common.BigToAddress(big.NewInt(1))
	txHash := common.BigToHash(big.NewInt(2))
	recipient := common.BigToAddress(big.NewInt(3))
	denom := "testdenom"

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
						Topics:  []common.Hash{keeper.NativeTransferEvent.ID},
					},
				}
				err := suite.app.EvmKeeper.PostTxProcessing(txHash, logs)
				suite.Require().NoError(err)
			},
		},
		{
			"not enough balance, expect fail",
			func() {
				data, err := keeper.NativeTransferEvent.Inputs.Pack(
					recipient,
					big.NewInt(100),
				)
				suite.Require().NoError(err)
				logs := []*ethtypes.Log{
					{
						Address: contract,
						Topics:  []common.Hash{keeper.NativeTransferEvent.ID},
						Data:    data,
					},
				}
				err = suite.app.EvmKeeper.PostTxProcessing(txHash, logs)
				suite.Require().Error(err)
			},
		},
		{
			"success native transfer",
			func() {
				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, denom, contract)
				coin := sdk.NewCoin(denom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(coin, balance)

				data, err := keeper.NativeTransferEvent.Inputs.Pack(
					recipient,
					coin.Amount.BigInt(),
				)
				suite.Require().NoError(err)
				logs := []*ethtypes.Log{
					{
						Address: contract,
						Topics:  []common.Hash{keeper.NativeTransferEvent.ID},
						Data:    data,
					},
				}
				err = suite.app.EvmKeeper.PostTxProcessing(txHash, logs)
				suite.Require().NoError(err)

				balance = suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(sdk.NewCoin(denom, sdk.NewInt(0)), balance)
				balance = suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(recipient.Bytes()), denom)
				suite.Require().Equal(coin, balance)
			},
		},
		{
			"failed ethereum transfer, invalid denom",
			func() {
				suite.SetupTest()

				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, denom, contract)
				coin := sdk.NewCoin(denom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(coin, balance)

				data, err := keeper.EthereumTransferEvent.Inputs.Pack(
					recipient,
					coin.Amount.BigInt(),
					big.NewInt(0),
				)
				suite.Require().NoError(err)
				logs := []*ethtypes.Log{
					{
						Address: contract,
						Topics:  []common.Hash{keeper.EthereumTransferEvent.ID},
						Data:    data,
					},
				}
				err = suite.app.EvmKeeper.PostTxProcessing(txHash, logs)
				// should fail, because of not gravity denom name
				suite.Require().Error(err)
			},
		},
		{
			"success ethereum transfer",
			func() {
				suite.SetupTest()
				denom := "gravity0x0000000000000000000000000000000000000000"

				suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, denom, contract)
				coin := sdk.NewCoin(denom, sdk.NewInt(100))
				err := suite.MintCoins(sdk.AccAddress(contract.Bytes()), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(coin, balance)

				data, err := keeper.EthereumTransferEvent.Inputs.Pack(
					recipient,
					coin.Amount.BigInt(),
					big.NewInt(0),
				)
				suite.Require().NoError(err)
				logs := []*ethtypes.Log{
					{
						Address: contract,
						Topics:  []common.Hash{keeper.EthereumTransferEvent.ID},
						Data:    data,
					},
				}
				err = suite.app.EvmKeeper.PostTxProcessing(txHash, logs)
				suite.Require().NoError(err)

				// sender's balance deducted
				balance = suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(sdk.NewCoin(denom, sdk.NewInt(0)), balance)
				// query unbatched SendToEthereum message exist
				rsp, err := suite.app.GravityKeeper.UnbatchedSendToEthereums(sdk.WrapSDKContext(suite.ctx), &gravitytypes.UnbatchedSendToEthereumsRequest{
					SenderAddress: sdk.AccAddress(contract.Bytes()).String(),
				})
				suite.Require().Equal(1, len(rsp.SendToEthereums))
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			tc.malleate()
		})
	}
}
