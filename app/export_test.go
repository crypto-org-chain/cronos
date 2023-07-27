package app

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"
)

func TestExportAppStateAndValidators(t *testing.T) {
	testCases := []struct {
		name          string
		forZeroHeight bool
	}{
		{
			"for zero height",
			true,
		},
		{
			"for non-zero height",
			false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			privKey, err := ethsecp256k1.GenerateKey()
			require.NoError(t, err)
			address := sdk.AccAddress(privKey.PubKey().Address())
			app := Setup(t, address.String(), true)
			app.Commit()
			_, err = app.ExportAppStateAndValidators(tc.forZeroHeight, []string{}, []string{})
			require.NoError(t, err, "ExportAppStateAndValidators should not have an error")
		})
	}
}
