package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"

	relayer "github.com/crypto-org-chain/cronos/relayer"
)

const (
	flagConfig = "config"
	flagHome   = "home"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "relayerd",
		Short: "Cronos Attestation Layer Relayer",
		Long: `Relayer service that connects Cronos EVM network with a Cosmos SDK-based 
attestation Layer 1 chain. Provides block forwarding, finality monitoring, 
and forced transaction execution.`,
		RunE: runRelayer,
	}

	rootCmd.Flags().String(flagConfig, "config.json", "Path to configuration file")
	rootCmd.Flags().String(flagHome, "", "Home directory for relayer data")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runRelayer(cmd *cobra.Command, args []string) error {
	// Create logger
	logger := log.NewLogger(os.Stdout)
	logger.Info("Starting Cronos Attestation Layer Relayer")

	// Load configuration
	configPath, _ := cmd.Flags().GetString(flagConfig)
	config, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger.Info("Configuration loaded",
		"source_chain", config.SourceChainID,
		"attestation_chain", config.AttestationChainID,
	)

	// Setup client contexts
	homeDir, _ := cmd.Flags().GetString(flagHome)
	if homeDir == "" {
		homeDir = os.ExpandEnv("$HOME/.relayer")
	}

	sourceClientCtx, err := createClientContext(
		config.SourceRPC,
		config.SourceGRPC,
		config.RelayerMnemonic,
		config.SourceChainID,
		homeDir,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create source client context: %w", err)
	}

	attestationClientCtx, err := createClientContext(
		config.AttestationRPC,
		config.AttestationGRPC,
		config.RelayerMnemonic,
		config.AttestationChainID,
		homeDir,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create attestation client context: %w", err)
	}

	// Create relayer service
	service, err := relayer.NewRelayerService(
		config,
		logger,
		sourceClientCtx,
		attestationClientCtx,
	)
	if err != nil {
		return fmt.Errorf("failed to create relayer service: %w", err)
	}

	// Start relayer service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := service.Start(ctx); err != nil {
		return fmt.Errorf("failed to start relayer service: %w", err)
	}

	logger.Info("Relayer service started successfully")

	// Print status periodically
	go printStatus(ctx, service, logger)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	logger.Info("Received interrupt signal, shutting down...")

	// Stop relayer service
	if err := service.Stop(); err != nil {
		logger.Error("Failed to stop relayer service", "error", err)
		return err
	}

	logger.Info("Relayer service stopped successfully")
	return nil
}

func loadConfig(path string) (*relayer.Config, error) {
	// Check if path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		config := relayer.DefaultConfig()

		// Override with environment variables if present
		if rpc := os.Getenv("SOURCE_RPC"); rpc != "" {
			config.SourceRPC = rpc
		}
		if grpc := os.Getenv("SOURCE_GRPC"); grpc != "" {
			config.SourceGRPC = grpc
		}
		if rpc := os.Getenv("ATTESTATION_RPC"); rpc != "" {
			config.AttestationRPC = rpc
		}
		if grpc := os.Getenv("ATTESTATION_GRPC"); grpc != "" {
			config.AttestationGRPC = grpc
		}
		if mnemonic := os.Getenv("RELAYER_MNEMONIC"); mnemonic != "" {
			config.RelayerMnemonic = mnemonic
		}

		return config, nil
	}

	// Load from file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config relayer.Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func createClientContext(
	rpcEndpoint string,
	grpcEndpoint string,
	mnemonic string,
	chainID string,
	homeDir string,
	logger log.Logger,
) (client.Context, error) {
	// Create codec
	interfaceRegistry := types.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	legacyAmino := codec.NewLegacyAmino()
	std.RegisterLegacyAminoCodec(legacyAmino)

	// Create keyring
	kb, err := keyring.New("relayer", keyring.BackendTest, homeDir, nil, marshaler)
	if err != nil {
		return client.Context{}, fmt.Errorf("failed to create keyring: %w", err)
	}

	// Import or get key from mnemonic
	keyName := "relayer"
	_, err = kb.Key(keyName)
	if err != nil {
		// Key doesn't exist, import from mnemonic if provided
		if mnemonic != "" {
			_, err = kb.NewAccount(keyName, mnemonic, "", sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1)
			if err != nil {
				return client.Context{}, fmt.Errorf("failed to import key from mnemonic: %w", err)
			}
			logger.Info("Imported relayer key", "key_name", keyName)
		} else {
			return client.Context{}, fmt.Errorf("relayer key not found and no mnemonic provided")
		}
	}

	// Get key info
	keyInfo, err := kb.Key(keyName)
	if err != nil {
		return client.Context{}, fmt.Errorf("failed to get key info: %w", err)
	}

	addr, err := keyInfo.GetAddress()
	if err != nil {
		return client.Context{}, fmt.Errorf("failed to get address: %w", err)
	}

	logger.Info("Using relayer address", "address", addr.String(), "chain_id", chainID)

	// Create RPC client
	rpcClient, err := rpchttp.New(rpcEndpoint, "/websocket")
	if err != nil {
		return client.Context{}, fmt.Errorf("failed to create RPC client: %w", err)
	}

	// Create client context
	clientCtx := client.Context{}.
		WithCodec(marshaler).
		WithInterfaceRegistry(interfaceRegistry).
		WithTxConfig(tx.NewTxConfig(marshaler, tx.DefaultSignModes)).
		WithLegacyAmino(legacyAmino).
		WithInput(os.Stdin).
		WithOutput(os.Stdout).
		WithKeyring(kb).
		WithClient(rpcClient).
		WithChainID(chainID).
		WithHomeDir(homeDir).
		WithFromName(keyName).
		WithFromAddress(addr).
		WithBroadcastMode(flags.BroadcastSync).
		WithSkipConfirmation(true)

	return clientCtx, nil
}

func printStatus(ctx context.Context, service *relayer.RelayerService, logger log.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := service.GetStatus()
			logger.Info("Relayer Status",
				"running", status.Running,
				"last_block_forwarded", status.LastBlockForwarded,
				"last_finality_received", status.LastFinalityReceived,
				"finalized_blocks_count", status.FinalizedBlocksCount,
			)

			// Get finality store stats
			stats, err := service.GetFinalityStoreStats()
			if err == nil && stats != nil {
				logger.Info("Finality Store Stats",
					"total_blocks", stats.TotalBlocks,
					"finalized_blocks", stats.FinalizedBlocks,
					"pending_blocks", stats.PendingBlocks,
					"latest_finalized", stats.LatestFinalized,
				)
			}
		}
	}
}
