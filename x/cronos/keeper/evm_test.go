package keeper_test

import (
	"math/big"

	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *KeeperTestSuite) TestDeployContract() {
	suite.SetupTest()
	keeper := suite.app.CronosKeeper

	_, err := keeper.DeployModuleCRC21(suite.ctx, "test")
	suite.Require().NoError(err)
}

func (suite *KeeperTestSuite) TestTokenConversion() {
	suite.SetupTest()
	keeper := suite.app.CronosKeeper

	// generate test address
	priv, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)
	address := common.BytesToAddress(priv.PubKey().Address().Bytes())
	cosmosAddress := sdk.AccAddress(address.Bytes())

	denom := "ibc/0000000000000000000000000000000000000000000000000000000000000000"
	amount := big.NewInt(100)
	coins := sdk.NewCoins(sdk.NewCoin(denom, sdkmath.NewIntFromBigInt(amount)))

	// mint native tokens
	err = suite.MintCoins(sdk.AccAddress(address.Bytes()), coins)
	suite.Require().NoError(err)

	// send to erc20
	err = keeper.ConvertCoinsFromNativeToCRC21(suite.ctx, address, coins, true)
	suite.Require().NoError(err)

	// check erc20 balance
	contract, found := keeper.GetContractByDenom(suite.ctx, denom)
	suite.Require().True(found)

	ret, err := keeper.CallModuleCRC21(suite.ctx, contract, "balanceOf", address)
	suite.Require().NoError(err)
	suite.Require().Equal(amount, big.NewInt(0).SetBytes(ret))

	ret, err = keeper.CallModuleCRC21(suite.ctx, contract, "totalSupply")
	suite.Require().NoError(err)
	suite.Require().Equal(amount, big.NewInt(0).SetBytes(ret))

	// convert back to native
	err = keeper.ConvertCoinFromCRC21ToNative(suite.ctx, contract, address, coins[0].Amount)
	suite.Require().NoError(err)

	ret, err = keeper.CallModuleCRC21(suite.ctx, contract, "balanceOf", address)
	suite.Require().NoError(err)
	suite.Require().Equal(0, big.NewInt(0).Cmp(big.NewInt(0).SetBytes(ret)))

	ret, err = keeper.CallModuleCRC21(suite.ctx, contract, "totalSupply")
	suite.Require().NoError(err)
	suite.Require().Equal(0, big.NewInt(0).Cmp(big.NewInt(0).SetBytes(ret)))

	// native balance recovered
	coin := suite.app.BankKeeper.GetBalance(suite.ctx, cosmosAddress, denom)
	suite.Require().Equal(amount, coin.Amount.BigInt())
}

func (suite *KeeperTestSuite) TestSourceTokenConversion() {
	suite.SetupTest()
	keeper := suite.app.CronosKeeper

	// generate test address
	priv, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)
	address := common.BytesToAddress(priv.PubKey().Address().Bytes())
	cosmosAddress := sdk.AccAddress(address.Bytes())

	// Deploy CRC21 token
	contractAddress, err := keeper.DeployModuleCRC21(suite.ctx, "Test")
	suite.Require().NoError(err)

	// Register the token
	denom := "cronos" + contractAddress.Hex()
	msgUpdateTokenMapping := types.MsgUpdateTokenMapping{
		Sender:   cosmosAddress.String(),
		Denom:    denom,
		Contract: contractAddress.Hex(),
		Symbol:   "Test",
		Decimal:  0,
	}
	err = keeper.RegisterOrUpdateTokenMapping(suite.ctx, &msgUpdateTokenMapping)
	suite.Require().NoError(err)

	// Mint some CRC21 token
	amount := big.NewInt(100)
	_, err = suite.app.CronosKeeper.CallModuleCRC21(suite.ctx, contractAddress, "mint_by_cronos_module", address, amount)
	suite.Require().NoError(err)

	// Convert CRC21 to native
	err = keeper.ConvertCoinFromCRC21ToNative(suite.ctx, contractAddress, address, sdkmath.NewIntFromBigInt(amount))
	suite.Require().NoError(err)

	// Check balance
	coin := suite.app.BankKeeper.GetBalance(suite.ctx, cosmosAddress, denom)
	suite.Require().Equal(amount, coin.Amount.BigInt())

	// Convert native to CRC21
	coins := sdk.NewCoins(sdk.NewCoin(denom, sdkmath.NewIntFromBigInt(amount)))
	err = keeper.ConvertCoinsFromNativeToCRC21(suite.ctx, address, coins, false)
	suite.Require().NoError(err)

	// check balance
	coin = suite.app.BankKeeper.GetBalance(suite.ctx, cosmosAddress, denom)
	suite.Require().Equal(big.NewInt(0), coin.Amount.BigInt())
	ret, err := keeper.CallModuleCRC21(suite.ctx, contractAddress, "balanceOf", address)
	suite.Require().NoError(err)
	suite.Require().Equal(0, big.NewInt(100).Cmp(big.NewInt(0).SetBytes(ret)))
}
