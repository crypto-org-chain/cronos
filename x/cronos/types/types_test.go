package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_IsValidIBCDenom(t *testing.T) {
	tests := []struct {
		name    string
		denom   string
		success bool
	}{
		{"wrong length", "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD", false},
		{"invalid denom", "aaa/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865", false},
		{"correct IBC denom", IbcCroDenomDefaultValue, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.success, IsValidIBCDenom(tt.denom))
		})
	}
}

func Test_IsValidGravityDenom(t *testing.T) {
	tests := []struct {
		name    string
		denom   string
		success bool
	}{
		{"wrong length", "gravity0x/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD", false},
		{"invalid denom", "aaa0xb7a4F3E9097C08dA09517b5aB877F7a917224ede", false},
		{"correct gravity denom", "gravity0xb7a4F3E9097C08dA09517b5aB877F7a917224ede", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.success, IsValidGravityDenom(tt.denom))
		})
	}
}

func Test_IsValidCronosDenom(t *testing.T) {
	tests := []struct {
		name    string
		denom   string
		success bool
	}{
		{"wrong length", "cronos0x/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD", false},
		{"invalid denom", "aaa0xb7a4F3E9097C08dA09517b5aB877F7a917224ede", false},
		{"correct cronos denom", "cronos0xb7a4F3E9097C08dA09517b5aB877F7a917224ede", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.success, IsValidCronosDenom(tt.denom))
		})
	}
}

func Test_IsValidCoinDenom(t *testing.T) {
	tests := []struct {
		name    string
		denom   string
		success bool
	}{
		{"wrong length", "ibc/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD", false},
		{"invalid denom", "aaa/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD5D19762F541EC971ACB0865", false},
		{"correct IBC denom", IbcCroDenomDefaultValue, true},
		{"wrong length", "gravity0x/6B5A664BF0AF4F71B2F0BAA33141E2F1321242FBD", false},
		{"invalid denom", "aaa0xb7a4F3E9097C08dA09517b5aB877F7a917224ede", false},
		{"correct gravity denom", "gravity0xb7a4F3E9097C08dA09517b5aB877F7a917224ede", true},
		{"correct cronos denom", "cronos0xb7a4F3E9097C08dA09517b5aB877F7a917224ede", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.success, IsValidCoinDenom(tt.denom))
		})
	}
}
