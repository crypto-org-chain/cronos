package preconfer

import (
	"fmt"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/types/mempool"
)

const (
	// PriorityNonceMempoolType is the expected type name for PriorityNonceMempool[int64]
	PriorityNonceMempoolType = "*mempool.PriorityNonceMempool[int64]"
)

// MempoolVerification contains detailed information about the mempool configuration
// returned by VerifyMempool function
type MempoolVerification struct {
	IsPreconferMempool          bool
	BaseMempoolType             string
	IsPriorityNonceMempool      bool
	PriorityBoost               int64
	SupportsInsertWithGasWanted bool
}

// VerifyMempool inspects a mempool and returns verification details
func VerifyMempool(m mempool.Mempool, logger log.Logger) MempoolVerification {
	verification := MempoolVerification{
		BaseMempoolType:             fmt.Sprintf("%T", m),
		SupportsInsertWithGasWanted: true, // All mempools should support this
	}

	// Check if it's a preconfer.Mempool
	if preconferMempool, ok := m.(*Mempool); ok {
		verification.IsPreconferMempool = true
		verification.BaseMempoolType = preconferMempool.GetBaseMempoolType()
		verification.IsPriorityNonceMempool = preconferMempool.IsPriorityNonceMempool()
		verification.PriorityBoost = preconferMempool.GetPriorityBoost()

		logger.Info("Mempool verification complete",
			"is_preconfer_mempool", verification.IsPreconferMempool,
			"base_mempool_type", verification.BaseMempoolType,
			"is_priority_nonce_mempool", verification.IsPriorityNonceMempool,
			"priority_boost", verification.PriorityBoost,
		)
	} else {
		// Not wrapped by preconfer.Mempool
		verification.IsPreconferMempool = false

		// Check if it's a direct PriorityNonceMempool
		typeName := fmt.Sprintf("%T", m)
		verification.IsPriorityNonceMempool = (typeName == PriorityNonceMempoolType)

		logger.Info("Mempool verification complete",
			"is_preconfer_mempool", false,
			"mempool_type", typeName,
			"is_priority_nonce_mempool", verification.IsPriorityNonceMempool,
		)
	}

	return verification
}

// ValidatePreconferMempool checks if the mempool is properly configured
// for preconfer functionality and returns an error if not
func ValidatePreconferMempool(m mempool.Mempool) error {
	preconferMempool, ok := m.(*Mempool)
	if !ok {
		return fmt.Errorf("mempool is not a preconfer.Mempool, got type: %T", m)
	}

	if !preconferMempool.IsPriorityNonceMempool() {
		return fmt.Errorf(
			"preconfer.Mempool is not wrapping a PriorityNonceMempool, got type: %s",
			preconferMempool.GetBaseMempoolType(),
		)
	}

	if preconferMempool.GetPriorityBoost() <= 0 {
		return fmt.Errorf(
			"preconfer.Mempool has invalid priority boost: %d",
			preconferMempool.GetPriorityBoost(),
		)
	}

	return nil
}

// LogMempoolConfiguration logs detailed mempool configuration for debugging
func LogMempoolConfiguration(m mempool.Mempool, logger log.Logger) {
	logger.Info("=== Mempool Configuration ===")
	logger.Info("Mempool type", "type", fmt.Sprintf("%T", m))

	if preconferMempool, ok := m.(*Mempool); ok {
		logger.Info("✓ Using preconfer.Mempool wrapper")
		logger.Info("Base mempool type", "type", preconferMempool.GetBaseMempoolType())
		logger.Info("Priority boost", "boost", preconferMempool.GetPriorityBoost())

		if preconferMempool.IsPriorityNonceMempool() {
			logger.Info("✓ Base mempool is PriorityNonceMempool[int64]")
			logger.Info("✓ Priority boosting will work correctly")
		} else {
			logger.Warn("⚠ Base mempool is NOT PriorityNonceMempool")
			logger.Warn("⚠ Priority boosting may not work as expected")
		}
	} else {
		logger.Info("Not using preconfer.Mempool wrapper")
		typeName := fmt.Sprintf("%T", m)
		if typeName == PriorityNonceMempoolType {
			logger.Info("Using direct PriorityNonceMempool (no priority boosting)")
		} else {
			logger.Info("Using mempool type", "type", typeName)
		}
	}
	logger.Info("=============================")
}
