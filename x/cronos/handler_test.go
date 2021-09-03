package cronos_test

import (
	"errors"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/crypto-org-chain/cronos/app"
	"github.com/crypto-org-chain/cronos/x/cronos"
	"github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/stretchr/testify/require"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"strings"
	"testing"
)

func TestInvalidMsg(t *testing.T) {
	app := app.Setup(false)
	handler := cronos.NewHandler(app.CronosKeeper)

	res, err := handler(sdk.NewContext(nil, tmproto.Header{}, false, nil), testdata.NewTestMsg())
	require.Error(t, err)
	require.Nil(t, res)

	_, _, log := sdkerrors.ABCIInfo(err, false)
	require.True(t, strings.Contains(log, "unrecognized cronos message type"))
}

func TestMsgConvertVouchers(t *testing.T) {
	testCases := []struct {
		name     string
		msg   *types.MsgConvertVouchers
		malleate   func()
		expectedError error
	}{
		{
			"Wrong address",
			types.NewMsgConvertVouchers("test", sdk.NewCoins(sdk.NewCoin("aphoton", sdk.NewInt(1)))),
			func(){},
			errors.New("decoding bech32 failed: invalid bech32 string length 4"),
		},
		{
			"Empty address",
			types.NewMsgConvertVouchers("", sdk.NewCoins(sdk.NewCoin("aphoton", sdk.NewInt(1)))),
			func(){},
			errors.New("empty address string is not allowed"),
		},
		{
			"Correct address with non supported coin denom",
			types.NewMsgConvertVouchers("eth1cml96vmptgw99syqrrz8az79xer2pcgpl6nuct", sdk.NewCoins(sdk.NewCoin("fake", sdk.NewInt(1)))),
			func(){},
			nil,
		},
	}

	app := app.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := cronos.NewHandler(app.CronosKeeper)
			_, err := handler(ctx, tc.msg)
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgTransferTokens(t *testing.T) {
	testCases := []struct {
		name     string
		msg   *types.MsgTransferTokens
		malleate   func()
		expectedError error
	}{
		{
			"Wrong from address",
			types.NewMsgTransferTokens("test", "to", sdk.NewCoins(sdk.NewCoin("aphoton", sdk.NewInt(1)))),
			func(){},
			errors.New("decoding bech32 failed: invalid bech32 string length 4"),
		},
		{
			"Empty from address",
			types.NewMsgTransferTokens("", "to", sdk.NewCoins(sdk.NewCoin("aphoton", sdk.NewInt(1)))),
			func(){},
			errors.New("empty address string is not allowed"),
		},
		{
			"Empty to address",
			types.NewMsgTransferTokens("eth1cml96vmptgw99syqrrz8az79xer2pcgpl6nuct", "", sdk.NewCoins(sdk.NewCoin("aphoton", sdk.NewInt(1)))),
			func(){},
			errors.New("to address cannot be empty"),
		},
		{
			"Correct address with non supported coin denom",
			types.NewMsgTransferTokens("eth1cml96vmptgw99syqrrz8az79xer2pcgpl6nuct", "to", sdk.NewCoins(sdk.NewCoin("fake", sdk.NewInt(1)))),
			func(){},
			nil,
		},
	}

	app := app.Setup(false)
	ctx := app.BaseApp.NewContext(false, tmproto.Header{})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handler := cronos.NewHandler(app.CronosKeeper)
			_, err := handler(ctx, tc.msg)
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}