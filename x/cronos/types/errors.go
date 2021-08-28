package types

// DONTCOVER

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	codeErrIbcCroDenomEmpty = uint32(iota) + 2 // NOTE: code 1 is reserved for internal errors
	codeErrConvertDisabled
)

// x/cronos module sentinel errors
var (
	ErrIbcCroDenomEmpty = sdkerrors.Register(ModuleName, codeErrIbcCroDenomEmpty, "ibc cro denom is not set")
	ErrConvertDisabled  = sdkerrors.Register(ModuleName, codeErrConvertDisabled, "convert transactions are disabled")
	// this line is used by starport scaffolding # ibc/errors
)
