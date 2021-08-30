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
	EventTypeConvertCoin   = "convert_coin"
	EventTypeSendCryptoOrg = "send_crypto_org"
)

// NewCoinSpentEvent constructs a new coin convert sdk.Event
// nolint: interfacer
func NewConvertCoinEvent(sender sdk.AccAddress, amount fmt.Stringer) sdk.Event {
	return sdk.NewEvent(
		EventTypeConvertCoin,
		sdk.NewAttribute(AttributeKeySender, sender.String()),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
	)
}

// NewSendToCryptoOrgEvent constructs a new sendToCryptoOrg convert sdk.Event
func NewSendToCryptoOrgEvent(sender string, recipient string, amount fmt.Stringer) sdk.Event {
	return sdk.NewEvent(
		EventTypeSendCryptoOrg,
		sdk.NewAttribute(AttributeKeySender, sender),
		sdk.NewAttribute(AttributeKeyRecipient, recipient),
		sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
	)
}
