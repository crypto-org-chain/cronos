package keeper_test

import (
	"fmt"
	"math/big"

	cronosmodulekeeper "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	handlers "github.com/crypto-org-chain/cronos/x/cronos/keeper/evmhandlers"
	keepertest "github.com/crypto-org-chain/cronos/x/cronos/keeper/mock"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
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
				data, err := handlers.SendToAccountEvent.Inputs.NonIndexed().Pack(
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
				err := suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, denom, contract)
				suite.Require().NoError(err)
				coin := sdk.NewCoin(denom, sdkmath.NewInt(100))
				err = suite.MintCoins(contract.Bytes(), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(coin, balance)

				data, err := handlers.SendToAccountEvent.Inputs.NonIndexed().Pack(
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
				suite.Require().Equal(sdk.NewCoin(denom, sdkmath.NewInt(0)), balance)
				balance = suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(recipient.Bytes()), denom)
				suite.Require().Equal(coin, balance)
			},
		},
		{
			"failed send to ibc, invalid ibc denom",
			func() {
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
				suite.app.CronosKeeper = cronosKeeper

				err := suite.app.CronosKeeper.SetExternalContractForDenom(suite.ctx, denom, contract)
				suite.Require().NoError(err)
				coin := sdk.NewCoin(denom, sdkmath.NewInt(100))
				err = suite.MintCoins(contract.Bytes(), sdk.NewCoins(coin))
				suite.Require().NoError(err)

				balance := suite.app.BankKeeper.GetBalance(suite.ctx, sdk.AccAddress(contract.Bytes()), denom)
				suite.Require().Equal(coin, balance)

				data, err := handlers.SendToIbcEvent.Inputs.NonIndexed().Pack(
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
