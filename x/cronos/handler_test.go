package cronos_test

import (
	"errors"
	"testing"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/crypto-org-chain/cronos/app"
	"github.com/crypto-org-chain/cronos/x/cronos"
	"github.com/crypto-org-chain/cronos/x/cronos/keeper"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type CronosTestSuite struct {
	suite.Suite

	ctx     sdk.Context
	app     *app.App
	address sdk.AccAddress
}

func TestCronosTestSuite(t *testing.T) {
	suite.Run(t, new(CronosTestSuite))
}

func (suite *CronosTestSuite) SetupTest() {
	checkTx := false
	privKey, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)
	suite.address = sdk.AccAddress(privKey.PubKey().Address())
	suite.app = app.Setup(suite.T(), suite.address.String())
	suite.ctx = suite.app.NewContext(checkTx).WithBlockHeader(tmproto.Header{Height: 1, ChainID: app.TestAppChainID, Time: time.Now().UTC()})
}

func (suite *CronosTestSuite) TestMsgConvertVouchers() {
	testCases := []struct {
		name          string
		msg           *types.MsgConvertVouchers
		malleate      func()
		expectedError error
	}{
		{
			"Wrong address",
			types.NewMsgConvertVouchers("test", sdk.NewCoins(sdk.NewCoin("aphoton", sdkmath.NewInt(1)))),
			func() {},
			errors.New("decoding bech32 failed: invalid bech32 string length 4"),
		},
		{
			"Empty address",
			types.NewMsgConvertVouchers("", sdk.NewCoins(sdk.NewCoin("aphoton", sdkmath.NewInt(1)))),
			func() {},
			errors.New("empty address string is not allowed"),
		},
		{
			"Correct address with non supported coin denom",
			types.NewMsgConvertVouchers(suite.address.String(), sdk.NewCoins(sdk.NewCoin("fake", sdkmath.NewInt(1)))),
			func() {},
			errors.New("coin fake is not supported for conversion"),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			msgSrv := keeper.NewMsgServerImpl(suite.app.CronosKeeper)
			_, err := msgSrv.ConvertVouchers(suite.ctx, tc.msg)
			if tc.expectedError != nil {
				suite.Require().EqualError(err, tc.expectedError.Error())
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *CronosTestSuite) TestMsgTransferTokens() {
	testCases := []struct {
		name          string
		msg           *types.MsgTransferTokens
		malleate      func()
		expectedError error
	}{
		{
			"Wrong from address",
			types.NewMsgTransferTokens("test", "to", sdk.NewCoins(sdk.NewCoin("aphoton", sdkmath.NewInt(1)))),
			func() {},
			errors.New("decoding bech32 failed: invalid bech32 string length 4"),
		},
		{
			"Empty from address",
			types.NewMsgTransferTokens("", "to", sdk.NewCoins(sdk.NewCoin("aphoton", sdkmath.NewInt(1)))),
			func() {},
			errors.New("empty address string is not allowed"),
		},
		{
			"Empty to address",
			types.NewMsgTransferTokens(suite.address.String(), "", sdk.NewCoins(sdk.NewCoin("aphoton", sdkmath.NewInt(1)))),
			func() {},
			errors.New("to address cannot be empty"),
		},
		{
			"Correct address with non supported coin denom",
			types.NewMsgTransferTokens(suite.address.String(), "to", sdk.NewCoins(sdk.NewCoin("fake", sdkmath.NewInt(1)))),
			func() {},
			errors.New("the coin fake is neither an ibc voucher or a cronos token"),
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			msgSrv := keeper.NewMsgServerImpl(suite.app.CronosKeeper)
			_, err := msgSrv.TransferTokens(suite.ctx, tc.msg)
			if tc.expectedError != nil {
				suite.Require().EqualError(err, tc.expectedError.Error())
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}

func (suite *CronosTestSuite) TestUpdateTokenMapping() {
	suite.SetupTest()

	contractAddr, err := suite.app.CronosKeeper.DeployModuleCRC21(suite.ctx, "Test")
	suite.Require().NoError(err)
	contract := contractAddr.Hex()
	denom := "gravity" + contract

	msg := types.NewMsgUpdateTokenMapping(suite.address.String(), denom, contract, "", 0)
	err = suite.app.CronosKeeper.RegisterOrUpdateTokenMapping(suite.ctx, msg)
	suite.Require().NoError(err)

	contractAddr, found := suite.app.CronosKeeper.GetContractByDenom(suite.ctx, denom)
	suite.Require().True(found)
	suite.Require().Equal(contract, contractAddr.Hex())
}

func (suite *CronosTestSuite) TestTokenMappingProposalHandlerValidateBasic() {
	suite.SetupTest()

	handler := cronos.NewTokenMappingChangeProposalHandler(suite.app.CronosKeeper)

	// invalid proposal (empty title) must be rejected
	suite.Run("invalid proposal rejected", func() {
		proposal := &types.TokenMappingChangeProposal{
			Title:       "",
			Description: "description",
			Denom:       "gravity0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883",
			Contract:    "0xF6D4FeCB1a6fb7C2CA350169A050D483bd87b883",
			Symbol:      "SYM",
			Decimal:     0,
		}
		err := handler(suite.ctx, proposal)
		suite.Require().Error(err)
	})

	// valid proposal with a deployed contract must succeed
	suite.Run("valid proposal accepted", func() {
		contractAddr, err := suite.app.CronosKeeper.DeployModuleCRC21(suite.ctx, "Test")
		suite.Require().NoError(err)

		proposal := &types.TokenMappingChangeProposal{
			Title:       "Map test token",
			Description: "description",
			Denom:       "gravity" + contractAddr.Hex(),
			Contract:    contractAddr.Hex(),
			Symbol:      "SYM",
			Decimal:     0,
		}
		err = handler(suite.ctx, proposal)
		suite.Require().NoError(err)
	})
}
