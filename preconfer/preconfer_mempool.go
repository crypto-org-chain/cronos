package preconfer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"cosmossdk.io/log"
	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

var _ mempool.Mempool = &Mempool{}

// Mempool wraps the standard mempool and enhances it
// to give marked transactions higher priority
type Mempool struct {
	mempool.Mempool
	txDecoder sdk.TxDecoder
	logger    log.Logger

	// Priority boost for marked transactions
	// This value is added to the transaction's base priority
	priorityBoost int64

	// Whitelist for priority boosting
	// If empty, all addresses can use priority boosting
	// If non-empty, only whitelisted addresses can boost priority
	whitelistMu sync.RWMutex
	whitelist   map[string]bool

	// SignerExtractor for extracting signer addresses from transactions
	signerExtractor mempool.SignerExtractionAdapter
}

// MempoolConfig configuration for the preconfer mempool
type MempoolConfig struct {
	// BaseMempool is the underlying mempool implementation
	BaseMempool mempool.Mempool

	// TxDecoder for decoding transaction bytes
	TxDecoder sdk.TxDecoder

	// PriorityBoost is the priority increase for marked transactions
	// Default is 1_000_000_000 if not specified
	PriorityBoost int64

	// Logger for mempool operations
	Logger log.Logger

	// WhitelistAddresses is the list of addresses allowed to boost priority
	// If empty, all addresses are allowed
	WhitelistAddresses []string

	// SignerExtractor for extracting signer addresses from transactions
	// If not provided, uses NewDefaultSignerExtractionAdapter
	SignerExtractor mempool.SignerExtractionAdapter
}

// NewMempool creates a new preconfer mempool
func NewMempool(cfg MempoolConfig) *Mempool {
	if cfg.PriorityBoost == 0 {
		cfg.PriorityBoost = DefaultPriorityBoost
	}

	if cfg.Logger == nil {
		cfg.Logger = log.NewNopLogger()
	}

	if cfg.SignerExtractor == nil {
		cfg.SignerExtractor = mempool.NewDefaultSignerExtractionAdapter()
	}

	// Initialize whitelist map
	whitelist := make(map[string]bool)
	for _, addr := range cfg.WhitelistAddresses {
		whitelist[addr] = true
	}

	m := &Mempool{
		Mempool:         cfg.BaseMempool,
		txDecoder:       cfg.TxDecoder,
		logger:          cfg.Logger,
		priorityBoost:   cfg.PriorityBoost,
		whitelist:       whitelist,
		signerExtractor: cfg.SignerExtractor,
	}

	// Log the base mempool type and whitelist configuration
	if len(whitelist) == 0 {
		cfg.Logger.Info("preconfer.Mempool initialized",
			"base_mempool_type", fmt.Sprintf("%T", cfg.BaseMempool),
			"priority_boost", cfg.PriorityBoost,
			"whitelist", "disabled (all addresses allowed)")
	} else {
		cfg.Logger.Info("preconfer.Mempool initialized",
			"base_mempool_type", fmt.Sprintf("%T", cfg.BaseMempool),
			"priority_boost", cfg.PriorityBoost,
			"whitelist", "enabled",
			"whitelist_count", len(whitelist))
	}

	return m
}

// Insert adds a transaction to the mempool with priority boosting.
// If the transaction is marked as a priority transaction, its priority
// is boosted by adding priorityBoost to the context's priority value.
func (epm *Mempool) Insert(ctx context.Context, tx sdk.Tx) error {
	// Extract gas limit from transaction if available
	var gasWanted uint64
	if gasTx, ok := tx.(interface{ GetGas() uint64 }); ok {
		gasWanted = gasTx.GetGas()
	}

	return epm.InsertWithGasWanted(ctx, tx, gasWanted)
}

// InsertWithGasWanted adds a transaction to the mempool with explicit gas wanted.
// If the transaction is marked as a priority transaction, its priority is boosted
// by modifying the context's priority before delegating to the base mempool.
//
// The PriorityNonceMempool retrieves priority using TxPriority.GetTxPriority(ctx, tx),
// which reads from ctx.Priority(). By modifying the context's priority here, we ensure
// the boosted priority is properly indexed in the underlying mempool's skip list.
//
// Priority boosting is only applied if:
// 1. The transaction is marked with PRIORITY: prefix
// 2. The sender is whitelisted (or whitelist is empty/disabled)
func (epm *Mempool) InsertWithGasWanted(ctx context.Context, tx sdk.Tx, gasWanted uint64) error {
	// Check if this is a priority transaction
	isPriority := IsMarkedPriorityTx(tx)

	if isPriority {
		// Check if sender is authorized to boost priority
		isAuthorized := epm.isAddressWhitelisted(tx)

		if isAuthorized {
			// Log priority transaction insertion
			if txWithMemo, ok := tx.(sdk.TxWithMemo); ok {
				epm.logger.Debug("inserting priority transaction",
					"memo", txWithMemo.GetMemo(),
					"gas", gasWanted,
					"boost", epm.priorityBoost)
			}

			// Boost the priority by modifying the context
			// The base mempool will read this boosted priority value
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			currentPriority := sdkCtx.Priority()
			boostedPriority := currentPriority + epm.priorityBoost

			// Create a new context with the boosted priority
			sdkCtx = sdkCtx.WithPriority(boostedPriority)
			ctx = sdkCtx
		} else {
			epm.logger.Debug("priority transaction rejected - sender not whitelisted",
				"tx", fmt.Sprintf("%v", tx))
		}
	}

	// Delegate to base mempool's InsertWithGasWanted
	// InsertWithGasWanted is part of the mempool.Mempool interface
	return epm.Mempool.InsertWithGasWanted(ctx, tx, gasWanted)
}

// Select returns an iterator of transactions in priority order
// Priority transactions will naturally come first due to their boosted priority
func (epm *Mempool) Select(ctx context.Context, txs [][]byte) mempool.Iterator {
	return epm.Mempool.Select(ctx, txs)
}

// CountTx returns the number of transactions in the mempool
func (epm *Mempool) CountTx() int {
	return epm.Mempool.CountTx()
}

// Remove removes a transaction from the mempool
func (epm *Mempool) Remove(tx sdk.Tx) error {
	return epm.Mempool.Remove(tx)
}

// GetPriorityBoost returns the configured priority boost value
func (epm *Mempool) GetPriorityBoost() int64 {
	return epm.priorityBoost
}

// SetPriorityBoost allows dynamic adjustment of the priority boost
func (epm *Mempool) SetPriorityBoost(boost int64) {
	if boost < 0 {
		epm.logger.Warn("attempted to set negative priority boost", "boost", boost)
		return
	}
	epm.priorityBoost = boost
	epm.logger.Info("priority boost updated", "new_boost", boost)
}

// GetStats returns mempool statistics
func (epm *Mempool) GetStats() string {
	return fmt.Sprintf("Mempool{count=%d, boost=%d}",
		epm.CountTx(), epm.priorityBoost)
}

// GetBaseMempool returns the underlying base mempool
// This is useful for type checking and verification
func (epm *Mempool) GetBaseMempool() mempool.Mempool {
	return epm.Mempool
}

// IsPriorityNonceMempool checks if the base mempool is a PriorityNonceMempool
func (epm *Mempool) IsPriorityNonceMempool() bool {
	typeName := fmt.Sprintf("%T", epm.Mempool)
	// Check if it's a PriorityNonceMempool (the type will be *mempool.PriorityNonceMempool[int64])
	return typeName == PriorityNonceMempoolType
}

// GetBaseMempoolType returns the type name of the base mempool
func (epm *Mempool) GetBaseMempoolType() string {
	return fmt.Sprintf("%T", epm.Mempool)
}

// isAddressWhitelisted checks if the transaction sender is authorized for priority boosting
// Returns true if:
// - Whitelist is empty (everyone allowed)
// - Sender address is in the whitelist
func (epm *Mempool) isAddressWhitelisted(tx sdk.Tx) bool {
	epm.whitelistMu.RLock()
	defer epm.whitelistMu.RUnlock()

	// If whitelist is empty, allow everyone
	if len(epm.whitelist) == 0 {
		return true
	}

	// Extract signer addresses from transaction
	signers, err := epm.signerExtractor.GetSigners(tx)
	if err != nil {
		epm.logger.Debug("failed to extract signers", "error", err)
		return false
	}

	if len(signers) == 0 {
		epm.logger.Debug("transaction has no signers")
		return false
	}

	// Check if the first signer is whitelisted
	// (following the same pattern as PriorityNonceMempool)
	firstSigner := signers[0].Signer

	// Convert AccAddress to Ethereum address format (0x...)
	// AccAddress in Cronos is the 20-byte Ethereum address
	ethAddr := common.BytesToAddress(firstSigner).Hex()

	// Also check bech32 format for backward compatibility
	bech32Addr := firstSigner.String()

	// Check both formats (case-insensitive for Ethereum addresses)
	for whitelistAddr := range epm.whitelist {
		// Check if it's an Ethereum address format (0x...)
		if strings.HasPrefix(strings.ToLower(whitelistAddr), "0x") {
			if strings.EqualFold(whitelistAddr, ethAddr) {
				return true
			}
		} else {
			// Bech32 format comparison
			if whitelistAddr == bech32Addr {
				return true
			}
		}
	}

	return false
}

// AddToWhitelist adds an address to the whitelist
func (epm *Mempool) AddToWhitelist(address string) {
	epm.whitelistMu.Lock()
	defer epm.whitelistMu.Unlock()

	epm.whitelist[address] = true
	epm.logger.Info("address added to whitelist",
		"address", address,
		"whitelist_count", len(epm.whitelist))
}

// RemoveFromWhitelist removes an address from the whitelist
func (epm *Mempool) RemoveFromWhitelist(address string) {
	epm.whitelistMu.Lock()
	defer epm.whitelistMu.Unlock()

	delete(epm.whitelist, address)
	epm.logger.Info("address removed from whitelist",
		"address", address,
		"whitelist_count", len(epm.whitelist))
}

// IsWhitelisted checks if an address is in the whitelist
func (epm *Mempool) IsWhitelisted(address string) bool {
	epm.whitelistMu.RLock()
	defer epm.whitelistMu.RUnlock()

	return epm.whitelist[address]
}

// GetWhitelist returns a copy of the current whitelist
func (epm *Mempool) GetWhitelist() []string {
	epm.whitelistMu.RLock()
	defer epm.whitelistMu.RUnlock()

	addresses := make([]string, 0, len(epm.whitelist))
	for addr := range epm.whitelist {
		addresses = append(addresses, addr)
	}
	return addresses
}

// ClearWhitelist removes all addresses from the whitelist
// After this, all addresses will be allowed to boost priority
func (epm *Mempool) ClearWhitelist() {
	epm.whitelistMu.Lock()
	defer epm.whitelistMu.Unlock()

	epm.whitelist = make(map[string]bool)
	epm.logger.Info("whitelist cleared - all addresses now allowed")
}

// WhitelistCount returns the number of addresses in the whitelist
func (epm *Mempool) WhitelistCount() int {
	epm.whitelistMu.RLock()
	defer epm.whitelistMu.RUnlock()

	return len(epm.whitelist)
}

// SetWhitelist replaces the entire whitelist with a new set of addresses
func (epm *Mempool) SetWhitelist(addresses []string) {
	epm.whitelistMu.Lock()
	defer epm.whitelistMu.Unlock()

	epm.whitelist = make(map[string]bool)
	for _, addr := range addresses {
		epm.whitelist[addr] = true
	}

	if len(addresses) == 0 {
		epm.logger.Info("whitelist set to empty - all addresses now allowed")
	} else {
		epm.logger.Info("whitelist updated",
			"whitelist_count", len(epm.whitelist))
	}
}
