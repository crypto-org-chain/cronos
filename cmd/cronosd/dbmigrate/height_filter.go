package dbmigrate

import (
	"bytes"
	"fmt"

	dbm "github.com/cosmos/cosmos-db"
)

// Database name constants
const (
	DBNameBlockstore = "blockstore"
	DBNameTxIndex    = "tx_index"
)

// HeightRange represents block heights to migrate
// Can be a continuous range or specific heights
type HeightRange struct {
	Start           int64   // inclusive, 0 means from beginning (only used for ranges)
	End             int64   // inclusive, 0 means to end (only used for ranges)
	SpecificHeights []int64 // specific heights to migrate (if set, Start/End are ignored)
}

// IsWithinRange checks if a height is within the specified range or in specific heights
func (hr HeightRange) IsWithinRange(height int64) bool {
	// If specific heights are set, check if height is in the list
	if len(hr.SpecificHeights) > 0 {
		for _, h := range hr.SpecificHeights {
			if h == height {
				return true
			}
		}
		return false
	}

	// Otherwise use range check
	if hr.Start > 0 && height < hr.Start {
		return false
	}
	if hr.End > 0 && height > hr.End {
		return false
	}
	return true
}

// IsEmpty returns true if no height range or specific heights are specified
func (hr HeightRange) IsEmpty() bool {
	return hr.Start == 0 && hr.End == 0 && len(hr.SpecificHeights) == 0
}

// HasSpecificHeights returns true if specific heights are specified (not a range)
func (hr HeightRange) HasSpecificHeights() bool {
	return len(hr.SpecificHeights) > 0
}

// String returns a human-readable representation of the height range
func (hr HeightRange) String() string {
	if hr.IsEmpty() {
		return "all heights"
	}

	// Specific heights
	if len(hr.SpecificHeights) > 0 {
		if len(hr.SpecificHeights) == 1 {
			return fmt.Sprintf("height %d", hr.SpecificHeights[0])
		}
		if len(hr.SpecificHeights) <= 5 {
			// Show all heights if 5 or fewer
			heightStrs := make([]string, len(hr.SpecificHeights))
			for i, h := range hr.SpecificHeights {
				heightStrs[i] = fmt.Sprintf("%d", h)
			}
			return fmt.Sprintf("heights %s", joinStrings(heightStrs, ", "))
		}
		// Show count if more than 5
		return fmt.Sprintf("%d specific heights", len(hr.SpecificHeights))
	}

	// Range
	if hr.Start > 0 && hr.End > 0 {
		return fmt.Sprintf("heights %d to %d", hr.Start, hr.End)
	}
	if hr.Start > 0 {
		return fmt.Sprintf("heights from %d", hr.Start)
	}
	if hr.End > 0 {
		return fmt.Sprintf("heights up to %d", hr.End)
	}
	return "all heights"
}

// joinStrings joins strings with a separator
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// Validate checks if the height range is valid
func (hr HeightRange) Validate() error {
	// Validate specific heights
	if len(hr.SpecificHeights) > 0 {
		for _, h := range hr.SpecificHeights {
			if h < 0 {
				return fmt.Errorf("height cannot be negative: %d", h)
			}
		}
		return nil
	}

	// Validate range
	if hr.Start < 0 {
		return fmt.Errorf("start height cannot be negative: %d", hr.Start)
	}
	if hr.End < 0 {
		return fmt.Errorf("end height cannot be negative: %d", hr.End)
	}
	if hr.Start > 0 && hr.End > 0 && hr.Start > hr.End {
		return fmt.Errorf("start height (%d) cannot be greater than end height (%d)", hr.Start, hr.End)
	}
	return nil
}

// ParseHeightFlag parses the --height flag value
// Supports:
// - Range: "10000-20000"
// - Single height: "123456"
// - Multiple heights: "123456,234567,999999"
func ParseHeightFlag(heightStr string) (HeightRange, error) {
	if heightStr == "" {
		return HeightRange{}, nil
	}

	// Check if it's a range (contains '-')
	if bytes.IndexByte([]byte(heightStr), '-') >= 0 {
		return parseHeightRange(heightStr)
	}

	// Check if it contains commas (multiple heights)
	if bytes.IndexByte([]byte(heightStr), ',') >= 0 {
		return parseSpecificHeights(heightStr)
	}

	// Single height
	height, err := parseInt64(heightStr)
	if err != nil {
		return HeightRange{}, fmt.Errorf("invalid height value: %w", err)
	}
	if height < 0 {
		return HeightRange{}, fmt.Errorf("height cannot be negative: %d", height)
	}

	return HeightRange{
		SpecificHeights: []int64{height},
	}, nil
}

// parseHeightRange parses a range like "10000-20000"
func parseHeightRange(rangeStr string) (HeightRange, error) {
	parts := splitString(rangeStr, '-')
	if len(parts) != 2 {
		return HeightRange{}, fmt.Errorf("invalid range format, expected 'start-end', got: %s", rangeStr)
	}

	start, err := parseInt64(trimSpace(parts[0]))
	if err != nil {
		return HeightRange{}, fmt.Errorf("invalid start height: %w", err)
	}

	end, err := parseInt64(trimSpace(parts[1]))
	if err != nil {
		return HeightRange{}, fmt.Errorf("invalid end height: %w", err)
	}

	if start < 0 || end < 0 {
		return HeightRange{}, fmt.Errorf("heights cannot be negative: %d-%d", start, end)
	}

	if start > end {
		return HeightRange{}, fmt.Errorf("start height (%d) cannot be greater than end height (%d)", start, end)
	}

	return HeightRange{
		Start: start,
		End:   end,
	}, nil
}

// parseSpecificHeights parses comma-separated heights like "123456,234567,999999"
func parseSpecificHeights(heightsStr string) (HeightRange, error) {
	parts := splitString(heightsStr, ',')
	heights := make([]int64, 0, len(parts))

	for _, part := range parts {
		part = trimSpace(part)
		if part == "" {
			continue
		}

		height, err := parseInt64(part)
		if err != nil {
			return HeightRange{}, fmt.Errorf("invalid height value '%s': %w", part, err)
		}

		if height < 0 {
			return HeightRange{}, fmt.Errorf("height cannot be negative: %d", height)
		}

		heights = append(heights, height)
	}

	if len(heights) == 0 {
		return HeightRange{}, fmt.Errorf("no valid heights specified")
	}

	return HeightRange{
		SpecificHeights: heights,
	}, nil
}

// Helper functions for parsing

func parseInt64(s string) (int64, error) {
	var result int64
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

func splitString(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}

// extractHeightFromBlockstoreKey extracts block height from CometBFT blockstore keys
// CometBFT blockstore key formats (string-encoded):
//   - "H:" + height (as string) - block metadata
//   - "P:" + height (as string) + ":" + part - block parts
//   - "C:" + height (as string) - block commit
//   - "SC:" + height (as string) - seen commit
//   - "BH:" + hash (as hex string) - block header by hash
//   - "BS:H" - block store height (metadata)
func extractHeightFromBlockstoreKey(key []byte) (int64, bool) {
	if len(key) < 3 {
		return 0, false
	}

	keyStr := string(key)

	// Check for different key prefixes
	switch {
	case bytes.HasPrefix(key, []byte("H:")):
		// Block meta: "H:" + height (string)
		heightStr := keyStr[2:]
		var height int64
		_, err := fmt.Sscanf(heightStr, "%d", &height)
		if err == nil {
			return height, true
		}
		return 0, false

	case bytes.HasPrefix(key, []byte("P:")):
		// Block parts: "P:" + height (string) + ":" + part
		// Extract height between "P:" and next ":"
		start := 2
		end := start
		for end < len(keyStr) && keyStr[end] != ':' {
			end++
		}
		if end > start {
			heightStr := keyStr[start:end]
			var height int64
			_, err := fmt.Sscanf(heightStr, "%d", &height)
			if err == nil {
				return height, true
			}
		}
		return 0, false

	case bytes.HasPrefix(key, []byte("C:")):
		// Block commit: "C:" + height (string)
		heightStr := keyStr[2:]
		var height int64
		_, err := fmt.Sscanf(heightStr, "%d", &height)
		if err == nil {
			return height, true
		}
		return 0, false

	case bytes.HasPrefix(key, []byte("SC:")):
		// Seen commit: "SC:" + height (string)
		heightStr := keyStr[3:]
		var height int64
		_, err := fmt.Sscanf(heightStr, "%d", &height)
		if err == nil {
			return height, true
		}
		return 0, false

	case bytes.HasPrefix(key, []byte("BH:")):
		// Block header by hash - no height information
		return 0, false

	default:
		// Other keys (like "BS:H" for metadata) don't have height, include them
		return 0, false
	}
}

// extractHeightFromTxIndexKey extracts height from transaction index keys
// CometBFT tx_index key formats:
//   - "tx.height/" + height (as string) + "/" + hash - transaction by height
//   - Other index keys may have height in different positions
func extractHeightFromTxIndexKey(key []byte) (int64, bool) {
	keyStr := string(key)

	// Look for "tx.height/" prefix
	if bytes.HasPrefix(key, []byte("tx.height/")) {
		// Format: "tx.height/{height}/{hash}"
		// Extract height which comes after "tx.height/" and before next "/"
		start := len("tx.height/")
		if len(keyStr) <= start {
			return 0, false
		}

		// Find the next "/" after the height
		end := start
		for end < len(keyStr) && keyStr[end] != '/' {
			end++
		}

		if end > start {
			heightStr := keyStr[start:end]
			var height int64
			_, err := fmt.Sscanf(heightStr, "%d", &height)
			if err == nil {
				return height, true
			}
		}
	}

	// For other tx_index keys, check if they contain height information
	// Some keys might have height encoded differently
	// For now, include all keys that don't match known patterns
	return 0, false
}

// shouldIncludeKey determines if a key should be included based on database type and height range
func shouldIncludeKey(key []byte, dbName string, heightRange HeightRange) bool {
	// If no height range specified, include all keys
	if heightRange.IsEmpty() {
		return true
	}

	var height int64
	var hasHeight bool

	switch dbName {
	case DBNameBlockstore:
		height, hasHeight = extractHeightFromBlockstoreKey(key)
	case DBNameTxIndex:
		height, hasHeight = extractHeightFromTxIndexKey(key)
	default:
		// For other databases, height filtering is not supported
		return true
	}

	// If key doesn't have height information, include it (likely metadata)
	if !hasHeight {
		return true
	}

	// Check if height is within range
	return heightRange.IsWithinRange(height)
}

// makeBlockstoreIteratorKey creates a blockstore key for iterator bounds (string-encoded)
func makeBlockstoreIteratorKey(prefix string, height int64) []byte {
	return []byte(fmt.Sprintf("%s%d", prefix, height))
}

// getBlockstoreIterators creates bounded iterators for blockstore database based on height range
// Returns a slice of iterators, one for each key prefix (H:, P:, C:, SC:)
func getBlockstoreIterators(db dbm.DB, heightRange HeightRange) ([]dbm.Iterator, error) {
	if heightRange.IsEmpty() {
		// No height filtering, return full iterator
		itr, err := db.Iterator(nil, nil)
		if err != nil {
			return nil, err
		}
		return []dbm.Iterator{itr}, nil
	}

	var iterators []dbm.Iterator
	prefixes := []string{"H:", "P:", "C:", "SC:"}

	// Determine start and end heights
	var startHeight, endHeight int64
	if heightRange.HasSpecificHeights() {
		// For specific heights, find min and max
		startHeight = heightRange.SpecificHeights[0]
		endHeight = heightRange.SpecificHeights[0]
		for _, h := range heightRange.SpecificHeights {
			if h < startHeight {
				startHeight = h
			}
			if h > endHeight {
				endHeight = h
			}
		}
	} else {
		// For range, use Start and End directly
		startHeight = heightRange.Start
		endHeight = heightRange.End
	}

	for _, prefix := range prefixes {
		var start, end []byte

		if startHeight > 0 {
			start = makeBlockstoreIteratorKey(prefix, startHeight)
		} else {
			// Start from the beginning of this prefix
			start = []byte(prefix)
		}

		if endHeight > 0 {
			// End is exclusive in Iterator, so we need to increment by 1
			end = makeBlockstoreIteratorKey(prefix, endHeight+1)
		} else {
			// Calculate the end of this prefix range
			// For "H:", next prefix would be "I:"
			// We can use prefix + 0xFF... to get to the end
			end = append([]byte(prefix), 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
		}

		itr, err := db.Iterator(start, end)
		if err != nil {
			// Close any previously opened iterators
			for _, it := range iterators {
				it.Close()
			}
			return nil, fmt.Errorf("failed to create iterator for prefix %s: %w", prefix, err)
		}
		iterators = append(iterators, itr)
	}

	return iterators, nil
}

// getTxIndexIterator creates a bounded iterator for tx_index database based on height range
func getTxIndexIterator(db dbm.DB, heightRange HeightRange) (dbm.Iterator, error) {
	if heightRange.IsEmpty() {
		// No height filtering, return full iterator
		return db.Iterator(nil, nil)
	}

	// For tx_index, we primarily care about tx.height/ keys
	// Format: "tx.height/{height}/{hash}"
	var start, end []byte

	// Determine start and end heights
	var startHeight, endHeight int64
	if heightRange.HasSpecificHeights() {
		// For specific heights, find min and max
		startHeight = heightRange.SpecificHeights[0]
		endHeight = heightRange.SpecificHeights[0]
		for _, h := range heightRange.SpecificHeights {
			if h < startHeight {
				startHeight = h
			}
			if h > endHeight {
				endHeight = h
			}
		}
	} else {
		// For range, use Start and End directly
		startHeight = heightRange.Start
		endHeight = heightRange.End
	}

	if startHeight > 0 {
		start = []byte(fmt.Sprintf("tx.height/%d/", startHeight))
	} else {
		start = []byte("tx.height/")
	}

	if endHeight > 0 {
		// We need to include all transactions at End height
		// So we go to the next height
		end = []byte(fmt.Sprintf("tx.height/%d/", endHeight+1))
	} else {
		// Go to the end of tx.height namespace
		end = []byte("tx.height/~") // ~ is after numbers and /
	}

	return db.Iterator(start, end)
}

// supportsHeightFiltering returns true if the database supports height-based filtering
func supportsHeightFiltering(dbName string) bool {
	return dbName == DBNameBlockstore || dbName == DBNameTxIndex
}
