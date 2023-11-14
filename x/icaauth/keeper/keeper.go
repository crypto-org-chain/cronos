package keeper

import (
	"context"
	"fmt"
	"time"

	errorsmod "cosmossdk.io/errors"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/controller/keeper"
	icatypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	porttypes "github.com/cosmos/ibc-go/v7/modules/core/05-port/types"
	host "github.com/cosmos/ibc-go/v7/modules/core/24-host"
	"github.com/crypto-org-chain/cronos/v2/x/icaauth/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type (
	Keeper struct {
		cdc      codec.Codec
		storeKey storetypes.StoreKey
		memKey   storetypes.StoreKey

		icaControllerKeeper    icacontrollerkeeper.Keeper
		ics4Wrapper            porttypes.ICS4Wrapper
		scopedKeeper           capabilitykeeper.ScopedKeeper
		controllerScopedKeeper capabilitykeeper.ScopedKeeper
	}
)

func NewKeeper(
	cdc codec.Codec,
	storeKey,
	memKey storetypes.StoreKey,
	icaControllerKeeper icacontrollerkeeper.Keeper,
	scopedKeeper capabilitykeeper.ScopedKeeper,
	controllerScopedKeeper capabilitykeeper.ScopedKeeper,
) *Keeper {
	return &Keeper{
		cdc:                    cdc,
		storeKey:               storeKey,
		memKey:                 memKey,
		icaControllerKeeper:    icaControllerKeeper,
		scopedKeeper:           scopedKeeper,
		controllerScopedKeeper: controllerScopedKeeper,
	}
}

func (k *Keeper) WithICS4Wrapper(ics4Wrapper porttypes.ICS4Wrapper) {
	k.ics4Wrapper = ics4Wrapper
}

// SubmitTx submits a transaction to the host chain on behalf of interchain account
func (k *Keeper) SubmitTx(goCtx context.Context, msg *types.MsgSubmitTx) (*types.MsgSubmitTxResponse, error) {
	msgs, err := msg.GetMessages()
	if err != nil {
		return nil, err
	}

	data, err := icatypes.SerializeCosmosTx(k.cdc, msgs)
	if err != nil {
		return nil, err
	}

	packetData := icatypes.InterchainAccountPacketData{
		Type: icatypes.EXECUTE_TX,
		Data: data,
	}
	_, rsp, err := k.SubmitTxWithArgs(goCtx, msg.Owner, msg.ConnectionId, *msg.TimeoutDuration, packetData)
	return rsp, err
}

func (k *Keeper) sendTx(ctx sdk.Context, connectionID, portID string, icaPacketData icatypes.InterchainAccountPacketData, timeoutTimestamp uint64) (string, uint64, error) {
	activeChannelID, found := k.icaControllerKeeper.GetOpenActiveChannel(ctx, connectionID, portID)
	if !found {
		return "", 0, errorsmod.Wrapf(icatypes.ErrActiveChannelNotFound, "failed to retrieve active channel on connection %s for port %s", connectionID, portID)
	}

	chanCap, found := k.controllerScopedKeeper.GetCapability(ctx, host.ChannelCapabilityPath(portID, activeChannelID))
	if !found {
		return "", 0, errorsmod.Wrapf(capabilitytypes.ErrCapabilityNotFound, "failed to find capability: %s", host.ChannelCapabilityPath(portID, activeChannelID))
	}

	if uint64(ctx.BlockTime().UnixNano()) >= timeoutTimestamp {
		return "", 0, icatypes.ErrInvalidTimeoutTimestamp
	}

	if err := icaPacketData.ValidateBasic(); err != nil {
		return "", 0, errorsmod.Wrap(err, "invalid interchain account packet data")
	}

	sequence, err := k.ics4Wrapper.SendPacket(ctx, chanCap, portID, activeChannelID, clienttypes.ZeroHeight(), timeoutTimestamp, icaPacketData.GetBytes())
	if err != nil {
		return "", 0, err
	}
	return activeChannelID, sequence, nil
}

func (k *Keeper) SubmitTxWithArgs(goCtx context.Context, owner, connectionId string, timeoutDuration time.Duration, packetData icatypes.InterchainAccountPacketData) (string, *types.MsgSubmitTxResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	portID, err := icatypes.NewControllerPortID(owner)
	if err != nil {
		return "", nil, err
	}
	minTimeoutDuration := k.MinTimeoutDuration(ctx)
	// timeoutDuration should be constraited by MinTimeoutDuration parameter.
	timeoutTimestamp := ctx.BlockTime().Add(
		types.MsgSubmitTx{
			TimeoutDuration: &timeoutDuration,
		}.CalculateTimeoutDuration(minTimeoutDuration)).UnixNano()
	activeChannelID, res, err := k.sendTx(ctx, connectionId, portID, packetData, uint64(timeoutTimestamp))
	if err != nil {
		return "", nil, err
	}
	return activeChannelID, &types.MsgSubmitTxResponse{
		Sequence: res,
	}, nil
}

// RegisterAccount registers an interchain account with the given `connectionId` and `owner` on the host chain
func (k *Keeper) RegisterAccount(goCtx context.Context, msg *types.MsgRegisterAccount) (*types.MsgRegisterAccountResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := k.icaControllerKeeper.RegisterInterchainAccount(ctx, msg.ConnectionId, msg.Owner, msg.Version); err != nil {
		return nil, err
	}
	return &types.MsgRegisterAccountResponse{}, nil
}

// InterchainAccountAddress fetches the interchain account address for given `connectionId` and `owner`
func (k Keeper) InterchainAccountAddress(goCtx context.Context, req *types.QueryInterchainAccountAddressRequest) (*types.QueryInterchainAccountAddressResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	portID, err := icatypes.NewControllerPortID(req.Owner)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid owner address: %s", err)
	}

	icaAddress, found := k.icaControllerKeeper.GetInterchainAccountAddress(ctx, req.ConnectionId, portID)

	if !found {
		return nil, status.Errorf(codes.NotFound, "could not find account")
	}

	return &types.QueryInterchainAccountAddressResponse{
		InterchainAccountAddress: icaAddress,
	}, nil
}

// ClaimCapability claims the channel capability passed via the OnOpenChanInit callback
func (k *Keeper) ClaimCapability(ctx sdk.Context, cap *capabilitytypes.Capability, name string) error {
	return k.scopedKeeper.ClaimCapability(ctx, cap, name)
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
