package keeper_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
)

func (suite *KeeperTestSuite) TestUpdateTokenMapping() {
	suite.SetupTest()

	denom := "gravity0x6E7eef2b30585B2A4D45Ba9312015d5354FDB067"
	contract := "0x57f96e6B86CdeFdB3d412547816a82E3E0EbF9D2"

	msg := types.NewMsgUpdateTokenMapping(sdk.AccAddress(suite.address.Bytes()).String(), denom, contract)
	msgServer := keeper.NewMsgServerImpl(suite.app.CronosKeeper)
	_, err := msgServer.UpdateTokenMapping(sdk.WrapSDKContext(suite.ctx), msg)
	suite.Require().NoError(err)

	contractAddr, found := suite.app.CronosKeeper.GetContractByDenom(suite.ctx, denom)
	suite.Require().True(found)
	suite.Require().Equal(contract, contractAddr.Hex())
}
