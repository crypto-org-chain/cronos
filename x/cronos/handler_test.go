package cronos_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	errorsmod "cosmossdk.io/errors"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/crypto-org-chain/cronos/v2/app"
	"github.com/crypto-org-chain/cronos/v2/x/cronos"
	"github.com/crypto-org-chain/cronos/v2/x/cronos/types"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/stretchr/testify/suite"
)

type CronosTestSuite struct {
	suite.Suite

	ctx     sdk.Context
	handler sdk.Handler
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
	suite.app = app.Setup(suite.T(), suite.address.String(), true)
	suite.ctx = suite.app.BaseApp.NewContext(checkTx, tmproto.Header{Height: 1, ChainID: app.TestAppChainID, Time: time.Now().UTC()})
	suite.handler = cronos.NewHandler(suite.app.CronosKeeper)
}

func (suite *CronosTestSuite) TestInvalidMsg() {
	res, err := suite.handler(sdk.NewContext(nil, tmproto.Header{}, false, nil), testdata.NewTestMsg())
	suite.Require().Error(err)
	suite.Nil(res)

	_, _, log := errorsmod.ABCIInfo(err, false)
	suite.Require().True(strings.Contains(log, "unrecognized cronos message type"))
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
			types.NewMsgConvertVouchers("test", sdk.NewCoins(sdk.NewCoin("aphoton", sdk.NewInt(1)))),
			func() {},
			errors.New("decoding bech32 failed: invalid bech32 string length 4"),
		},
		{
			"Empty address",
			types.NewMsgConvertVouchers("", sdk.NewCoins(sdk.NewCoin("aphoton", sdk.NewInt(1)))),
			func() {},
			errors.New("empty address string is not allowed"),
		},
		{
			"Correct address with non supported coin denom",
			types.NewMsgConvertVouchers(suite.address.String(), sdk.NewCoins(sdk.NewCoin("fake", sdk.NewInt(1)))),
			func() {},
			errors.New("coin fake is not supported for conversion"),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			handler := cronos.NewHandler(suite.app.CronosKeeper)
			_, err := handler(suite.ctx, tc.msg)
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
			types.NewMsgTransferTokens("test", "to", sdk.NewCoins(sdk.NewCoin("aphoton", sdk.NewInt(1)))),
			func() {},
			errors.New("decoding bech32 failed: invalid bech32 string length 4"),
		},
		{
			"Empty from address",
			types.NewMsgTransferTokens("", "to", sdk.NewCoins(sdk.NewCoin("aphoton", sdk.NewInt(1)))),
			func() {},
			errors.New("empty address string is not allowed"),
		},
		{
			"Empty to address",
			types.NewMsgTransferTokens(suite.address.String(), "", sdk.NewCoins(sdk.NewCoin("aphoton", sdk.NewInt(1)))),
			func() {},
			errors.New("to address cannot be empty"),
		},
		{
			"Correct address with non supported coin denom",
			types.NewMsgTransferTokens(suite.address.String(), "to", sdk.NewCoins(sdk.NewCoin("fake", sdk.NewInt(1)))),
			func() {},
			errors.New("the coin fake is neither an ibc voucher or a cronos token"),
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			handler := cronos.NewHandler(suite.app.CronosKeeper)
			_, err := handler(suite.ctx, tc.msg)
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

	denom := "gravity0x6E7eef2b30585B2A4D45Ba9312015d5354FDB067"
	contract := "0x57f96e6B86CdeFdB3d412547816a82E3E0EbF9D2"

	msg := types.NewMsgUpdateTokenMapping(suite.address.String(), denom, contract, "", 0)
	handler := cronos.NewHandler(suite.app.CronosKeeper)
	_, err := handler(suite.ctx, msg)
	suite.Require().NoError(err)

	contractAddr, found := suite.app.CronosKeeper.GetContractByDenom(suite.ctx, denom)
	suite.Require().True(found)
	suite.Require().Equal(contract, contractAddr.Hex())
}
