package config

import (
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetBech32Prefixes sets the global prefixes to be used when serializing addresses and public keys to Bech32 strings.
func SetBech32Prefixes(config *sdk.Config) {
	config.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
}

type CronosConfig struct {
	// Set to true to disable tx replacement.
	DisableTxReplacement bool `mapstructure:"disable-tx-replacement"`
	// Set to true to disable optimistic execution.
	DisableOptimisticExecution bool `mapstructure:"disable-optimistic-execution"`
}

const (
	NodeTypeDefault   = ""
	NodeTypeValidator = "validator"
	NodeTypeRPC       = "rpc"
	NodeTypeArchive   = "archive"
)

type RocksDBConfig struct {
	// Defines the tuning profile for RocksDB based on the node's primary workload.
	// Valid values: "", "validator", "rpc", "archive"
	NodeType string `mapstructure:"node_type"`
}

func (c *RocksDBConfig) Validate() error {
	normalized := strings.ToLower(strings.TrimSpace(c.NodeType))
	switch normalized {
	case NodeTypeDefault, NodeTypeValidator, NodeTypeRPC, NodeTypeArchive:
		c.NodeType = normalized
		return nil
	default:
		return fmt.Errorf("invalid rocksdb.node_type %q: allowed values are %q, %q, %q, or %q (empty)",
			c.NodeType, NodeTypeValidator, NodeTypeRPC, NodeTypeArchive, NodeTypeDefault)
	}
}

func DefaultCronosConfig() CronosConfig {
	return CronosConfig{
		DisableTxReplacement:       false,
		DisableOptimisticExecution: false,
	}
}

func DefaultRocksDBConfig() RocksDBConfig {
	return RocksDBConfig{
		NodeType: "",
	}
}
