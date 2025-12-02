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
	_ module.AppModule      = (*AppModule)(nil)
	_ module.AppModuleBasic = (*AppModuleBasic)(nil)
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
	keeper keeper.Keeper
}

// NewAppModule creates a new AppModule object
func NewAppModule(cdc codec.Codec, k keeper.Keeper) AppModule {
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
func (am AppModule) InitGenesis(ctx context.Context, cdc codec.JSONCodec, data json.RawMessage) {
	var gs types.GenesisState
	if err := json.Unmarshal(data, &gs); err != nil {
		panic(err)
	}

	// Store genesis state
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Set v2 client ID if provided
	if gs.V2ClientID != "" {
		if err := am.keeper.SetV2ClientID(ctx, "attestation-layer", gs.V2ClientID); err != nil {
			panic(err)
		}
	}

	// Set last sent height
	if err := am.keeper.SetLastSentHeight(ctx, gs.LastSentHeight); err != nil {
		panic(err)
	}

	// Emit genesis event
	sdkCtx.EventManager().EmitEvent(
		sdk.NewEvent(
			"attestation_genesis_init",
			sdk.NewAttribute("attestation_enabled", fmt.Sprintf("%v", gs.Params.AttestationEnabled)),
			sdk.NewAttribute("v2_client_id", gs.V2ClientID),
			sdk.NewAttribute("last_sent_height", fmt.Sprintf("%d", gs.LastSentHeight)),
		),
	)
}

// ExportGenesis returns the exported genesis state for the attestation module
func (am AppModule) ExportGenesis(ctx context.Context, cdc codec.JSONCodec) json.RawMessage {
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

	// Get v2 client ID
	v2ClientID, _ := am.keeper.GetV2ClientID(ctx, "attestation-layer")

	// Get pending attestations
	// TODO: Implement GetPendingAttestations if needed
	pendingRecords := []*types.PendingAttestationRecord{}

	return &types.GenesisState{
		Params:              params,
		LastSentHeight:      lastSentHeight,
		PendingAttestations: pendingRecords,
		V2ClientID:          v2ClientID,
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

	if !params.AttestationEnabled {
		return nil
	}

	currentHeight := uint64(sdkCtx.BlockHeight())

	// Get last sent height
	lastSentHeight, err := am.keeper.GetLastSentHeight(ctx)
	if err != nil {
		return err
	}

	// Send attestation if it's time
	if currentHeight > lastSentHeight && (currentHeight-lastSentHeight >= params.AttestationInterval) {
		// Collect attestations for blocks since last sent
		startHeight := lastSentHeight + 1
		endHeight := currentHeight

		// Limit by interval
		if endHeight-startHeight > params.AttestationInterval {
			endHeight = startHeight + params.AttestationInterval - 1
		}

		attestations, err := am.collectBlockAttestations(ctx, startHeight, endHeight)
		if err != nil || len(attestations) == 0 {
			am.keeper.Logger(ctx).Error("failed to collect block attestations",
				"start_height", startHeight,
				"end_height", endHeight,
				"error", err,
			)
			return nil // Don't fail the block
		}

		// Get v2 client ID for attestation layer
		v2ClientID, err := am.keeper.GetV2ClientID(ctx, "attestation-layer")
		if err != nil {
			am.keeper.Logger(ctx).Error("failed to get v2 client ID",
				"error", err,
			)
			return nil // Don't fail the block
		}

		// Send packet via IBC v2
		// IBC v2 protocol handles authentication automatically at transport layer
		_, err = am.keeper.SendAttestationPacketV2(
			ctx,
			am.keeper.ChainID(), // Source chain ID
			v2ClientID,          // Destination client ID
			attestations,
		)
		if err != nil {
			am.keeper.Logger(ctx).Error("failed to send attestation packet",
				"start_height", startHeight,
				"end_height", endHeight,
				"error", err,
			)
			return nil // Don't fail the block
		}

		// Update last sent height
		if err := am.keeper.SetLastSentHeight(ctx, endHeight); err != nil {
			return err
		}

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
func (am AppModule) collectBlockAttestations(ctx context.Context, startHeight, endHeight uint64) ([]*types.BlockAttestationData, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	var attestations []*types.BlockAttestationData

	for height := startHeight; height <= endHeight; height++ {
		// Get block info from context header
		blockInfo := sdkCtx.BlockHeader()

		// Create simplified attestation with available data
		// TODO: In production, we need to fetch full block data from CometBFT RPC
		attestation := &types.BlockAttestationData{
			BlockHeight:           height,
			BlockHash:             blockInfo.AppHash, // Use AppHash from header
			BlockHeader:           []byte{},          // TODO: Marshal full block header
			Transactions:          []byte{},          // TODO: Encode transactions
			TxResults:             []byte{},          // TODO: Encode tx results
			FinalizeBlockEvents:   []byte{},          // TODO: Encode events
			ValidatorUpdates:      []byte{},          // TODO: Encode validator updates
			ConsensusParamUpdates: []byte{},          // TODO: Encode consensus params
			Evidence:              []byte{},          // TODO: Encode evidence
			LastCommit:            []byte{},          // TODO: Encode last commit
		}

		attestations = append(attestations, attestation)
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
