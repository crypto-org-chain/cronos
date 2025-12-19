package attestation

import (
	"context"
	"encoding/json"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"github.com/crypto-org-chain/cronos/x/attestation/keeper"
	"github.com/crypto-org-chain/cronos/x/attestation/types"
)

var (
	_ module.AppModule        = (*AppModule)(nil)
	_ module.AppModuleBasic   = (*AppModuleBasic)(nil)
	_ module.HasGenesisBasics = (*AppModuleBasic)(nil)
	_ module.HasGenesis       = (*AppModule)(nil)
)

// AppModuleBasic defines the basic application module used by the attestation module
type AppModuleBasic struct {
	cdc codec.Codec
}

// Name returns the attestation module's name
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers the attestation module's types on the LegacyAmino codec
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	// Register amino codec if needed
}

// RegisterInterfaces registers the module's interface types
func (AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	// Register interface types if needed
}

// DefaultGenesis returns default genesis state as raw bytes for the attestation module
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	// GenesisState is not a proto type, marshal manually
	defaultGen := types.DefaultGenesis()
	bz, err := json.Marshal(defaultGen)
	if err != nil {
		panic(err)
	}
	return bz
}

// ValidateGenesis performs genesis state validation for the attestation module
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var gs types.GenesisState
	if err := json.Unmarshal(bz, &gs); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return gs.Validate()
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for the attestation module
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	if err := types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
}

// GetTxCmd returns the root tx command for the attestation module
func (am AppModuleBasic) GetTxCmd() *cobra.Command {
	return nil // No CLI tx commands for now
}

// GetQueryCmd returns the root query command for the attestation module
func (am AppModuleBasic) GetQueryCmd() *cobra.Command {
	return nil // No CLI query commands for now
}

// AppModule implements an application module for the attestation module
type AppModule struct {
	AppModuleBasic
	keeper *keeper.Keeper
}

// NewAppModule creates a new AppModule object
func NewAppModule(cdc codec.Codec, k *keeper.Keeper) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{cdc: cdc},
		keeper:         k,
	}
}

// Name returns the attestation module's name
func (AppModule) Name() string {
	return types.ModuleName
}

// InitGenesis performs genesis initialization for the attestation module
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var gs types.GenesisState
	if err := json.Unmarshal(data, &gs); err != nil {
		panic(err)
	}

	am.keeper.Logger(ctx).Info("attestation InitGenesis called",
		"v2_client_id", gs.V2ClientID,
		"last_sent_height", gs.LastSentHeight,
		"params", gs.Params,
		"ibc_version", am.keeper.GetIBCVersion(),
		"raw_data_length", len(data),
	)

	// TODO: why is v2 client id cant be set via a tx?
	gs.V2ClientID = "07-tendermint-0"

	// Set v2 client ID if provided
	if gs.V2ClientID != "" {
		am.keeper.Logger(ctx).Info("Setting v2 client ID in genesis",
			"key", "attestation-layer",
			"client_id", gs.V2ClientID,
		)
		if err := am.keeper.SetV2ClientID(ctx, "attestation-layer", gs.V2ClientID); err != nil {
			panic(err)
		}
		am.keeper.Logger(ctx).Info("Successfully set v2 client ID")
	} else if am.keeper.GetIBCVersion() == "v2" {
		am.keeper.Logger(ctx).Warn("v2 client ID is empty in genesis, attestation sending will be disabled until it's set")
	}

	// V1 channel ID and port ID are NOT set from genesis
	// They will be automatically discovered via IBC callbacks (OnChanOpenAck/OnChanOpenConfirm)
	// when the channel is created by a relayer or via CLI
	if am.keeper.GetIBCVersion() == "v1" {
		am.keeper.Logger(ctx).Info(
			"IBC v1 mode enabled. Channel ID and Port ID will be set automatically " +
				"via IBC callbacks when channel is created",
		)
	}

	// Set last sent height
	if err := am.keeper.SetLastSentHeight(ctx, gs.LastSentHeight); err != nil {
		panic(err)
	}

	// Emit genesis event
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			"attestation_genesis_init",
			sdk.NewAttribute("attestation_enabled", fmt.Sprintf("%v", gs.Params.AttestationEnabled)),
			sdk.NewAttribute("ibc_version", am.keeper.GetIBCVersion()),
			sdk.NewAttribute("v2_client_id", gs.V2ClientID),
			sdk.NewAttribute("last_sent_height", fmt.Sprintf("%d", gs.LastSentHeight)),
		),
	)
}

// ExportGenesis returns the exported genesis state for the attestation module
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := am.ExportGenesisState(ctx)
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(err)
	}
	return bz
}

// ExportGenesisState exports the genesis state
func (am AppModule) ExportGenesisState(ctx context.Context) *types.GenesisState {
	// Get last sent height
	lastSentHeight, _ := am.keeper.GetLastSentHeight(ctx)

	// Get params
	params, _ := am.keeper.GetParams(ctx)

	// Get v2 client ID (explicitly configured)
	v2ClientID, _ := am.keeper.GetV2ClientID(ctx, "attestation-layer")

	// V1 channel/port IDs are NOT exported - they are discovered dynamically
	// via IBC callbacks when channels are created

	// Get pending attestations
	// TODO: Implement GetPendingAttestations if needed
	pendingRecords := []*types.PendingAttestationRecord{}

	return &types.GenesisState{
		Params:              params,
		LastSentHeight:      lastSentHeight,
		PendingAttestations: pendingRecords,
		V2ClientID:          v2ClientID,
		// V1ChannelID and V1PortID removed - discovered via callbacks
	}
}

// RegisterInvariants registers the attestation module invariants
func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {}

// RegisterServices registers module services
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterQueryServer(cfg.QueryServer(), am.keeper)
}

// ConsensusVersion implements AppModule/ConsensusVersion
func (AppModule) ConsensusVersion() uint64 { return 1 }

// IsOnePerModuleType implements the depinject.OnePerModuleType interface
func (am AppModule) IsOnePerModuleType() {}

// IsAppModule implements the appmodule.AppModule interface
func (am AppModule) IsAppModule() {}

// GenerateGenesisState creates a randomized GenState of the attestation module
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
}

// RegisterStoreDecoder registers a decoder for attestation module's types
func (am AppModule) RegisterStoreDecoder(sdr simtypes.StoreDecoderRegistry) {}

// WeightedOperations returns the all the attestation module operations with their respective weights
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	return nil
}

// endBlocker is called at the end of every block
func (am AppModule) endBlocker(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	params, err := am.keeper.GetParams(ctx)
	if err != nil {
		return err
	}

	am.keeper.Logger(ctx).Info("attestation params", "params", params)

	if !params.AttestationEnabled {
		return nil
	}

	currentHeight := uint64(sdkCtx.BlockHeight())

	// Get last sent height
	lastSentHeight, err := am.keeper.GetLastSentHeight(ctx)
	if err != nil {
		return err
	}

	am.keeper.Logger(ctx).Info("last sent height", "last_sent_height", lastSentHeight)
	// Send attestation if it's time
	if currentHeight > lastSentHeight && (currentHeight-lastSentHeight >= params.AttestationInterval) {
		am.keeper.Logger(ctx).Info("sending attestation", "current_height", currentHeight, "last_sent_height", lastSentHeight)
		// Check if collector is available
		if am.keeper.BlockCollector == nil {
			am.keeper.Logger(ctx).Error("Block collector not initialized, skipping attestation")
			am.keeper.Logger(ctx).Debug("Block collector not initialized, skipping attestation")
			return nil
		}

		// Collect attestations for blocks since last sent
		startHeight := lastSentHeight + 1
		endHeight := currentHeight

		am.keeper.Logger(ctx).Info("start and end height", "start_height", startHeight, "end_height", endHeight)

		// Limit by interval
		if endHeight-startHeight > params.AttestationInterval {
			endHeight = startHeight + params.AttestationInterval - 1
		}

		// Only try to collect blocks that are recent (within last 100 blocks)
		// This avoids trying to collect very old blocks before collector started
		if currentHeight > 100 && startHeight < currentHeight-100 {
			startHeight = currentHeight - 100
			am.keeper.Logger(ctx).Debug("Adjusted start height to recent blocks",
				"original_start", lastSentHeight+1,
				"adjusted_start", startHeight,
				"current_height", currentHeight,
			)
		}

		am.keeper.Logger(ctx).Info("collecting block attestations", "start_height", startHeight, "end_height", endHeight)

		attestations, err := am.collectBlockAttestations(ctx, startHeight, endHeight)
		if err != nil || len(attestations) == 0 {
			am.keeper.Logger(ctx).Debug("Block data not available yet, skipping attestation",
				"start_height", startHeight,
				"end_height", endHeight,
				"error", err,
			)
			// Update last sent height to current so we don't keep trying to collect old blocks
			if err := am.keeper.SetLastSentHeight(ctx, currentHeight); err != nil {
				am.keeper.Logger(ctx).Error("failed to update last sent height", "error", err)
			}
			return nil // Don't fail the block - collector might still be starting
		}

		am.keeper.Logger(ctx).Info("collected block attestations", "attestations", len(attestations))

		am.keeper.Logger(ctx).Info("preparing to send attestation packet",
			"start_height", startHeight,
			"end_height", endHeight,
			"chain_id", am.keeper.ChainID(),
			"ibc_version", am.keeper.GetIBCVersion(),
		)

		// Dispatch to appropriate IBC version based on keeper configuration
		var sequence uint64
		var sendError error
		if am.keeper.GetIBCVersion() == "v1" {
			// Use IBC v1 (traditional port/channel)
			v1PortID, err := am.keeper.GetV1PortID(ctx, "attestation-layer")
			if err != nil {
				am.keeper.Logger(ctx).Debug("v1 port ID not configured, using default",
					"default_port", types.PortID,
					"error", err,
				)
				v1PortID = types.PortID
			}

			v1ChannelID, err := am.keeper.GetV1ChannelID(ctx, "attestation-layer")
			if err != nil {
				am.keeper.Logger(ctx).Info("v1 channel ID not configured yet, skipping attestation send",
					"key", "attestation-layer",
					"error", err,
				)
				return nil
			}

			am.keeper.Logger(ctx).Info("Retrieved v1 channel configuration",
				"port_id", v1PortID,
				"channel_id", v1ChannelID,
			)

			sequence, sendError = am.keeper.SendAttestationPacketV1(
				ctx,
				v1PortID,
				v1ChannelID,
				attestations,
			)
		} else {
			// Get v2 client ID for attestation layer
			v2ClientID, err := am.keeper.GetV2ClientID(ctx, "attestation-layer")
			am.keeper.Logger(ctx).Info("v2 client ID", "v2_client_id", v2ClientID)
			if err != nil {
				am.keeper.Logger(ctx).Debug("v2 client ID not configured yet, skipping attestation send",
					"error", err,
				)
				return nil
			}
			// Use IBC v2 (client-to-client)
			sequence, sendError = am.keeper.SendAttestationPacketV2(
				ctx,
				v2ClientID,
				v2ClientID, // destination client ID is the same as source client ID for testing
				attestations,
			)
		}

		if sendError != nil {
			am.keeper.Logger(ctx).Error("failed to send attestation packet",
				"start_height", startHeight,
				"end_height", endHeight,
				"ibc_version", am.keeper.GetIBCVersion(),
				"error", err,
			)
			return nil // Don't fail the block
		}

		am.keeper.Logger(ctx).Info("attestation packet sent successfully",
			"sequence", sequence,
			"ibc_version", am.keeper.GetIBCVersion(),
		)

		// Update last sent height
		if err := am.keeper.SetLastSentHeight(ctx, endHeight); err != nil {
			return err
		}

		am.keeper.Logger(ctx).Info("last sent height updated", "last_sent_height", endHeight)

		// Emit event
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				"attestation_sent",
				sdk.NewAttribute("start_height", fmt.Sprintf("%d", startHeight)),
				sdk.NewAttribute("end_height", fmt.Sprintf("%d", endHeight)),
				sdk.NewAttribute("count", fmt.Sprintf("%d", len(attestations))),
			),
		)
	}
	return nil
}

// collectBlockAttestations collects block attestation data for the specified height range
// This retrieves pre-collected block data from the BlockDataCollector
func (am AppModule) collectBlockAttestations(ctx context.Context, startHeight, endHeight uint64) ([]*types.BlockAttestationData, error) {
	// Use the block collector to get full block data
	// The collector subscribes to block events and stores complete block data
	// including headers, transactions, results, evidence, etc.

	if am.keeper.BlockCollector == nil {
		return nil, fmt.Errorf("block data collector not initialized")
	}

	attestations, err := am.keeper.BlockCollector.GetBlockDataRange(startHeight, endHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to collect block data range %d-%d: %w", startHeight, endHeight, err)
	}

	am.keeper.Logger(ctx).Info("collected block attestations from collector",
		"start_height", startHeight,
		"end_height", endHeight,
		"count", len(attestations),
	)

	for _, attestation := range attestations {
		am.keeper.Logger(ctx).Info("GONNA SEND block attestation data",
			"height", attestation.BlockHeight,
			"block_hash_len", len(attestation.BlockHash),
			"block_header_len", len(attestation.BlockHeader),
			"validator_updates_len", len(attestation.ValidatorUpdates),
			"consensus_params_len", len(attestation.ConsensusParamUpdates),
			"evidence_len", len(attestation.Evidence),
			"last_commit_len", len(attestation.LastCommit),
		)
	}

	return attestations, nil
}

// EndBlock implements the EndBlocker interface
func (am AppModule) EndBlock(ctx context.Context) ([]abci.ValidatorUpdate, error) {
	if err := am.endBlocker(ctx); err != nil {
		return nil, err
	}
	return []abci.ValidatorUpdate{}, nil
}

// No AutoCLIOptions for now
