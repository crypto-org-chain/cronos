package keeper

import (
	"fmt"
	"time"

	"cosmossdk.io/errors"
	"github.com/cometbft/cometbft/libs/log"
	"github.com/cosmos/cosmos-sdk/codec"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	capabilitytypes "github.com/cosmos/cosmos-sdk/x/capability/types"
	"github.com/cosmos/gogoproto/proto"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/controller/keeper"
	icatypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
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

		icaControllerKeeper icacontrollerkeeper.Keeper
		scopedKeeper        capabilitykeeper.ScopedKeeper
	}
)

func NewKeeper(
	cdc codec.Codec,
	storeKey,
	memKey storetypes.StoreKey,
	icaControllerKeeper icacontrollerkeeper.Keeper,
	scopedKeeper capabilitykeeper.ScopedKeeper,
) *Keeper {
	return &Keeper{
		cdc:      cdc,
		storeKey: storeKey,
		memKey:   memKey,

		icaControllerKeeper: icaControllerKeeper,
		scopedKeeper:        scopedKeeper,
	}
}

// DoSubmitTx submits a transaction to the host chain on behalf of interchain account
func (k *Keeper) DoSubmitTx(ctx sdk.Context, connectionID, owner string, msgs []proto.Message, timeoutDuration time.Duration) error {
	portID, err := icatypes.NewControllerPortID(owner)
	if err != nil {
		return err
	}

	channelID, found := k.icaControllerKeeper.GetActiveChannelID(ctx, connectionID, portID)
	if !found {
		return errors.Wrapf(icatypes.ErrActiveChannelNotFound, "failed to retrieve active channel for port %s", portID)
	}

	channelCapability, found := k.scopedKeeper.GetCapability(ctx, host.ChannelCapabilityPath(portID, channelID))
	if !found {
		return errors.Wrap(channeltypes.ErrChannelCapabilityNotFound, "module does not own channel capability")
	}

	data, err := icatypes.SerializeCosmosTx(k.cdc, msgs)
	if err != nil {
		return err
	}

	packetData := icatypes.InterchainAccountPacketData{
		Type: icatypes.EXECUTE_TX,
		Data: data,
	}

	// timeoutDuration should be constraited by MinTimeoutDuration parameter.
	timeoutTimestamp := ctx.BlockTime().Add(timeoutDuration).UnixNano()

	_, err = k.icaControllerKeeper.SendTx(ctx, channelCapability, connectionID, portID, packetData, uint64(timeoutTimestamp)) //nolint:staticcheck
	if err != nil {
		return err
	}

	return nil
}

// RegisterInterchainAccount registers an interchain account with the given `connectionId` and `owner` on the host chain
func (k *Keeper) RegisterInterchainAccount(ctx sdk.Context, connectionID, owner, version string) error {
	return k.icaControllerKeeper.RegisterInterchainAccount(ctx, connectionID, owner, version)
}

// GetInterchainAccountAddress fetches the interchain account address for given `connectionId` and `owner`
func (k *Keeper) GetInterchainAccountAddress(ctx sdk.Context, connectionID, owner string) (string, error) {
	portID, err := icatypes.NewControllerPortID(owner)
	if err != nil {
		return "", status.Errorf(codes.InvalidArgument, "invalid owner address: %s", err)
	}

	icaAddress, found := k.icaControllerKeeper.GetInterchainAccountAddress(ctx, connectionID, portID)

	if !found {
		return "", status.Errorf(codes.NotFound, "could not find account")
	}

	return icaAddress, nil
}

// ClaimCapability claims the channel capability passed via the OnOpenChanInit callback
func (k *Keeper) ClaimCapability(ctx sdk.Context, cap *capabilitytypes.Capability, name string) error {
	return k.scopedKeeper.ClaimCapability(ctx, cap, name)
}

func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}
