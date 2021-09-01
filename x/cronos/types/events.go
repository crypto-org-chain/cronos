package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	AttributeValueCategory = ModuleName

	AttributeKeyRecipient = "recipient"
	AttributeKeySender    = "sender"

	// events
	EventTypeConvertVouchers = "convert_vouchers"
	EventTypeTransferTokens  = "transfer_tokens"
)

// NewConvertVouchersEvent constructs a new voucher convert sdk.Event
// nolint: interfacer
func NewConvertVouchersEvent(sender string, amount fmt.Stringer) sdk.Event {
	return sdk.NewEvent(
		EventTypeConvertVouchers,
		sdk.NewAttribute(AttributeKeySender, sender),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
	)
}

// NewTransferTokensEvent constructs a new transfer sdk.Event
func NewTransferTokensEvent(sender string, recipient string, amount fmt.Stringer) sdk.Event {
	return sdk.NewEvent(
		EventTypeTransferTokens,
		sdk.NewAttribute(AttributeKeySender, sender),
		sdk.NewAttribute(AttributeKeyRecipient, recipient),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
	)
}
