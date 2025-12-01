package types

// FinalityStats contains statistics about finality storage
type FinalityStats struct {
	// Total number of finalized blocks
	TotalFinalized uint64 `json:"total_finalized"`

	// Minimum finalized block height
	MinHeight uint64 `json:"min_height"`

	// Maximum finalized block height
	MaxHeight uint64 `json:"max_height"`

	// Current cache size
	CacheSize uint64 `json:"cache_size"`

	// Maximum cache size
	CacheMaxSize uint64 `json:"cache_max_size"`
}



