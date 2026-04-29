package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

const (
	codeErrIbcCroDenomEmpty = uint32(iota) + 2 // NOTE: code 1 is reserved for internal errors
	codeErrIbcCroDenomInvalid
	codeErrExternalMappingExists
	codeErrContractAlreadyRegistered
	codeErrDenomAlreadyMapped
	codeErrSourceDenomContractMismatch
)

// x/cronos module sentinel errors
var (
	ErrIbcCroDenomEmpty            = errors.Register(ModuleName, codeErrIbcCroDenomEmpty, "ibc cro denom is not set")
	ErrIbcCroDenomInvalid          = errors.Register(ModuleName, codeErrIbcCroDenomInvalid, "ibc cro denom is invalid")
	ErrExternalMappingExists       = errors.Register(ModuleName, codeErrExternalMappingExists, "external mapping already exists")
	ErrContractAlreadyRegistered   = errors.Register(ModuleName, codeErrContractAlreadyRegistered, "contract already registered")
	ErrDenomAlreadyMapped          = errors.Register(ModuleName, codeErrDenomAlreadyMapped, "denom already mapped")
	ErrSourceDenomContractMismatch = errors.Register(
		ModuleName,
		codeErrSourceDenomContractMismatch,
		"source denom contract mismatch",
	)
	// this line is used by starport scaffolding # ibc/errors
)
