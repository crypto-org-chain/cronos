package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_validateIsIbcDenomParam(t *testing.T) {
	type args struct {
		i interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"invalid type", args{sdk.OneInt()}, true},

		{"wrong length", args{"ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD"}, true},
		{"invalid denom", args{"aaa/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865"}, true},
		{"correct IBC denom", args{IbcCroDenomDefaultValue}, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantErr, validateIsIbcDenom(tt.args.i) != nil)
		})
	}
}
