package types

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
		{"invalid type", args{sdkmath.OneInt()}, true},

		{"wrong length", args{"ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD"}, true},
		{"invalid denom", args{"aaa/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865"}, true},
		{"correct IBC denom", args{IbcCroDenomDefaultValue}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantErr, validateIsIbcDenom(tt.args.i) != nil)
		})
	}
}

func Test_validateIsUint64(t *testing.T) {
	type args struct {
		i interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"invalid type", args{"a"}, true},
		{"correct IBC timeout", args{IbcTimeoutDefaultValue}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantErr, validateIsUint64(tt.args.i) != nil)
		})
	}
}

func Test_validateIsBool(t *testing.T) {
	type args struct {
		i interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"invalid bool", args{"a"}, true},
		{"correct bool", args{true}, false},
		{"correct bool", args{false}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantErr, validateIsBool(tt.args.i) != nil)
		})
	}
}

func Test_validateIsAddress(t *testing.T) {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("crc", "crc"+sdk.PrefixPublic)

	type args struct {
		i interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"invalid address", args{"a"}, true},
		{"invalid bech32 prefix", args{"tcrc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp"}, true},
		{"correct bech32 address", args{"crc12luku6uxehhak02py4rcz65zu0swh7wjsrw0pp"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantErr, validateIsAddress(tt.args.i) != nil)
		})
	}
}

func Test_validateIbcTimeout(t *testing.T) {
	type args struct {
		i interface{}
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"invalid type", args{"a"}, true},
		{"zero is valid", args{uint64(0)}, false},
		{"default", args{IbcTimeoutDefaultValue}, false},
		{"at max", args{MaxIbcTimeoutValue}, false},
		{"above max", args{MaxIbcTimeoutValue + 1}, true},
		{"max uint64 wraps the timeout into the past", args{uint64(math.MaxUint64)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantErr, validateIbcTimeout(tt.args.i) != nil)
		})
	}
}

func Test_validateIbcTimeoutCannotOverflowBlockTime(t *testing.T) {
	require.NoError(t, validateIbcTimeout(MaxIbcTimeoutValue))
	require.Greater(t, uint64(math.MaxUint64)-MaxIbcTimeoutValue, uint64(time.Now().UnixNano()))
}

func Test_ParamsValidateRejectsOversizedIbcTimeout(t *testing.T) {
	params := DefaultParams()
	params.IbcTimeout = math.MaxUint64
	require.Error(t, params.Validate())

	params.IbcTimeout = IbcTimeoutDefaultValue
	require.NoError(t, params.Validate())
}
