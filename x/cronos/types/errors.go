package types

// DONTCOVER

import (
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// x/cronos module sentinel errors
var (
	ErrSample = sdkerrors.Register(ModuleName, 1100, "sample error")
	ErrConvertDisabled          = sdkerrors.Register(ModuleName, 5, "convert transactions are disabled")
	// this line is used by starport scaffolding # ibc/errors
)
