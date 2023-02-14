package types

// DONTCOVER

import (
	sdkerrors "cosmossdk.io/errors"
)

const (
	codeErrIbcCroDenomEmpty = uint32(iota) + 2 // NOTE: code 1 is reserved for internal errors
	codeErrIbcCroDenomInvalid
)

// x/cronos module sentinel errors
var (
	ErrIbcCroDenomEmpty   = sdkerrors.Register(ModuleName, codeErrIbcCroDenomEmpty, "ibc cro denom is not set")
	ErrIbcCroDenomInvalid = sdkerrors.Register(ModuleName, codeErrIbcCroDenomInvalid, "ibc cro denom is invalid")
	// this line is used by starport scaffolding # ibc/errors
)
