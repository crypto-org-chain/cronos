package middleware_test

import (
	"errors"
	"testing"
	"time"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	transferTypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v10/modules/core/exported"
	"github.com/crypto-org-chain/cronos/app"
	cronosmiddleware "github.com/crypto-org-chain/cronos/x/cronos/middleware"
	cronostypes "github.com/crypto-org-chain/cronos/x/cronos/types"
	"github.com/evmos/ethermint/crypto/ethsecp256k1"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// noopIBCModule is a stub porttypes.IBCModule used to isolate middleware behavior.
type noopIBCModule struct{}

var _ porttypes.IBCModule = noopIBCModule{}

func (noopIBCModule) OnChanOpenInit(sdk.Context, channeltypes.Order, []string, string, string, channeltypes.Counterparty, string) (string, error) {
	return "", nil
}

func (noopIBCModule) OnChanOpenTry(sdk.Context, channeltypes.Order, []string, string, string, channeltypes.Counterparty, string) (string, error) {
	return "", nil
}

func (noopIBCModule) OnChanOpenAck(sdk.Context, string, string, string, string) error {
	return nil
}
func (noopIBCModule) OnChanOpenConfirm(sdk.Context, string, string) error  { return nil }
func (noopIBCModule) OnChanCloseInit(sdk.Context, string, string) error    { return nil }
func (noopIBCModule) OnChanCloseConfirm(sdk.Context, string, string) error { return nil }

func (noopIBCModule) OnRecvPacket(sdk.Context, string, channeltypes.Packet, sdk.AccAddress) exported.Acknowledgement {
	return channeltypes.NewResultAcknowledgement([]byte("ok"))
}

func (noopIBCModule) OnAcknowledgementPacket(sdk.Context, string, channeltypes.Packet, []byte, sdk.AccAddress) error {
	return nil
}

func (noopIBCModule) OnTimeoutPacket(sdk.Context, string, channeltypes.Packet, sdk.AccAddress) error {
	return nil
}

func setupMiddlewareTest(t *testing.T) (cronosmiddleware.IBCConversionModule, sdk.Context, sdk.AccAddress, sdk.AccAddress) {
	t.Helper()

	senderPriv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	sender := sdk.AccAddress(senderPriv.PubKey().Address())

	receiverPriv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	receiver := sdk.AccAddress(receiverPriv.PubKey().Address())

	testApp := app.Setup(t, sender.String())
	ctx := testApp.NewContext(false).WithBlockHeader(tmproto.Header{
		Height:  1,
		ChainID: app.TestAppChainID,
		Time:    time.Now().UTC(),
	})

	im := cronosmiddleware.NewIBCConversionModule(noopIBCModule{}, testApp.CronosKeeper)
	return im, ctx, sender, receiver
}

func buildRefundPacket(t *testing.T, sender, receiver sdk.AccAddress) channeltypes.Packet {
	t.Helper()
	data := transferTypes.NewFungibleTokenPacketData(
		cronostypes.IbcCroDenomDefaultValue,
		"100",
		sender.String(),
		receiver.String(),
		"",
	)
	return channeltypes.NewPacket(
		data.GetBytes(),
		1,
		"transfer", "channel-0",
		"transfer", "channel-1",
		clienttypes.NewHeight(0, 100),
		0,
	)
}

// Ack refund path: sender has no IBC balance, so voucher conversion inside
// OnRecvVouchers fails. The middleware must log and continue; the underlying
// transfer module's ack result (nil here) is what is returned — the conversion
// error must not surface and block the refund.
func TestIBCConversionMiddleware_OnAcknowledgementPacket_RefundConversionFailure(t *testing.T) {
	im, ctx, sender, receiver := setupMiddlewareTest(t)
	packet := buildRefundPacket(t, sender, receiver)

	errAck := channeltypes.NewErrorAcknowledgement(errors.New("packet failed"))
	ackBz, err := transferTypes.ModuleCdc.MarshalJSON(&errAck)
	require.NoError(t, err)

	err = im.OnAcknowledgementPacket(ctx, transferTypes.V1, packet, ackBz, sdk.AccAddress{})
	require.NoError(t, err, "refund ack path must not propagate conversion error")
}

// Timeout refund path: same log-and-continue contract as the ack path.
func TestIBCConversionMiddleware_OnTimeoutPacket_RefundConversionFailure(t *testing.T) {
	im, ctx, sender, receiver := setupMiddlewareTest(t)
	packet := buildRefundPacket(t, sender, receiver)

	err := im.OnTimeoutPacket(ctx, transferTypes.V1, packet, sdk.AccAddress{})
	require.NoError(t, err, "refund timeout path must not propagate conversion error")
}
