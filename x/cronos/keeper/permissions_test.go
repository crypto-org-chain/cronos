package keeper_test

import (
	. "github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *KeeperTestSuite) TestSetAndGetPermissions() {
	suite.SetupTest()
	keeper := suite.app.CronosKeeper

	// generate test address
	priv, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)
	address := common.BytesToAddress(priv.PubKey().Address().Bytes())
	cosmosAddress := sdk.AccAddress(address.Bytes())

	permissions := keeper.GetPermissions(suite.ctx, cosmosAddress)
	suite.Require().Equal(uint64(0), permissions)

	keeper.SetPermissions(suite.ctx, cosmosAddress, CanChangeTokenMapping)
	permissions = keeper.GetPermissions(suite.ctx, cosmosAddress)
	suite.Require().Equal(CanChangeTokenMapping, permissions)

	keeper.SetPermissions(suite.ctx, cosmosAddress, CanTurnBridge)
	permissions = keeper.GetPermissions(suite.ctx, cosmosAddress)
	suite.Require().Equal(CanTurnBridge, permissions)

	keeper.SetPermissions(suite.ctx, cosmosAddress, All)
	permissions = keeper.GetPermissions(suite.ctx, cosmosAddress)
	suite.Require().Equal(All, permissions)
}

func (suite *KeeperTestSuite) TestHasPermissions() {
	suite.SetupTest()
	keeper := suite.app.CronosKeeper

	// generate test address
	priv, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)
	address := common.BytesToAddress(priv.PubKey().Address().Bytes())
	cosmosAddress := []sdk.AccAddress{sdk.AccAddress(address.Bytes())}

	suite.Require().Equal(true, keeper.HasPermission(suite.ctx, cosmosAddress, 0))
	suite.Require().Equal(true, keeper.HasPermission(suite.ctx, cosmosAddress, 0))

	suite.Require().Equal(false, keeper.HasPermission(suite.ctx, cosmosAddress, CanChangeTokenMapping))
	suite.Require().Equal(false, keeper.HasPermission(suite.ctx, cosmosAddress, CanTurnBridge))

	keeper.SetPermissions(suite.ctx, cosmosAddress[0], CanChangeTokenMapping)
	suite.Require().Equal(true, keeper.HasPermission(suite.ctx, cosmosAddress, CanChangeTokenMapping))
	suite.Require().Equal(false, keeper.HasPermission(suite.ctx, cosmosAddress, CanTurnBridge))

	keeper.SetPermissions(suite.ctx, cosmosAddress[0], CanTurnBridge)
	suite.Require().Equal(false, keeper.HasPermission(suite.ctx, cosmosAddress, CanChangeTokenMapping))
	suite.Require().Equal(true, keeper.HasPermission(suite.ctx, cosmosAddress, CanTurnBridge))

	keeper.SetPermissions(suite.ctx, cosmosAddress[0], All)
	suite.Require().Equal(true, keeper.HasPermission(suite.ctx, cosmosAddress, CanChangeTokenMapping))
	suite.Require().Equal(true, keeper.HasPermission(suite.ctx, cosmosAddress, CanTurnBridge))
}
