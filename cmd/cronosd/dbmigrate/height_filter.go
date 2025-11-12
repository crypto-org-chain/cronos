package dbmigrate

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

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
			return fmt.Sprintf("heights %s", strings.Join(heightStrs, ", "))
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
// ParseHeightFlag parses a --height flag value and returns the corresponding HeightRange.
// 
// Supported formats:
// - range: "start-end" (both start and end must be >= 0 and start <= end)
// - single height: "N" (N must be >= 0)
// - multiple heights: "a,b,c" (each height must be >= 0)
// An empty string yields an empty HeightRange. Returns an error for invalid formats or negative values.
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

// parseHeightRange parses a height range of the form "start-end" and returns a
// HeightRange with Start and End set. It validates the input format, ensures
// both heights are non-negative, and requires that Start is less than or equal
// to End; otherwise it returns a descriptive error.
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

// parseSpecificHeights parses a comma-separated list of block heights and returns a HeightRange containing those heights.
// It trims whitespace, requires each entry to be a non-negative integer, and returns an error if any entry is invalid or if no valid heights are provided.
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

// parseInt64 parses s as a base-10 int64 after trimming surrounding whitespace.
// It returns the parsed integer or an error if s is not a valid base-10 representation.

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

// splitString splits s using the single-byte separator sep and returns the resulting substrings.
func splitString(s string, sep byte) []string {
	return strings.Split(s, string(sep))
}

// trimSpace trims leading and trailing whitespace from s.
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// extractHeightFromBlockstoreKey extracts block height from CometBFT blockstore keys
// CometBFT blockstore key formats (string-encoded):
//   - "H:" + height (as string) - block metadata
//   - "P:" + height (as string) + ":" + part - block parts
//   - "C:" + height (as string) - block commit
//   - "SC:" + height (as string) - seen commit
//   - "BH:" + hash (as hex string) - block header by hash
// extractHeightFromBlockstoreKey parses a blockstore key and returns the block height encoded in it, if present.
// It recognizes keys with the prefixes "H:", "P:", "C:", and "SC:" and extracts the numeric height that follows.
// For "P:" it reads the number between "P:" and the next ":"; "BH:" and other keys do not contain height information.
// The function returns the parsed height and true on success, or 0 and false if no valid height can be extracted.
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
// extractHeightFromTxIndexKey extracts the block height encoded in a tx_index key when present.
// 
// It recognizes keys that begin with the "tx.height/" prefix and parses the numeric height that
// appears immediately after that prefix and before the next '/' (format: "tx.height/{height}/{hash}").
 // Returns the parsed height and true on successful extraction, or 0 and false if no height is found
// or parsing fails.
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

// shouldIncludeKey reports whether the provided DB key should be included according to the given heightRange.
// It extracts a block height from keys for supported databases (blockstore and tx_index) and returns true
// for keys that have no height information or for databases that do not support height filtering. If a height
// is found, it returns whether that height falls within heightRange.
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

// makeBlockstoreIteratorKey creates a blockstore iterator bound key by concatenating the prefix and the base-10 encoding of height.
func makeBlockstoreIteratorKey(prefix string, height int64) []byte {
	return []byte(fmt.Sprintf("%s%d", prefix, height))
}

// getBlockstoreIterators creates bounded iterators for blockstore database based on height range
// getBlockstoreIterators returns a slice of iterators covering blockstore keys for the prefixes "H:", "P:", "C:", and "SC:" according to the provided HeightRange.
// 
// If heightRange is empty, a single full-range iterator is returned. For a non-empty heightRange the function computes start and end bounds from either the continuous Start/End fields or the min/max of SpecificHeights and opens one bounded iterator per prefix. End bounds are treated as exclusive. If creating any iterator fails, previously opened iterators are closed and the error is returned.
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

// getTxIndexIterator creates a DB iterator over tx_index entries constrained to the provided HeightRange.
// If the range is empty, it returns an iterator for the full tx_index namespace.
// For a specified range or specific heights it computes a start and end key in the form "tx.height/{height}/",
// where the end key is exclusive (uses endHeight+1) so that all entries at endHeight are included.
// When no startHeight is set the iterator starts at "tx.height/"; when no endHeight is set it ends at "tx.height/~".
// It returns an iterator that covers matching tx.height entries or an error if creating the iterator fails.
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

// supportsHeightFiltering reports whether the named database supports height-based filtering.
func supportsHeightFiltering(dbName string) bool {
	return dbName == DBNameBlockstore || dbName == DBNameTxIndex
}