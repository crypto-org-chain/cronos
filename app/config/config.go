package config

import "github.com/evmos/ethermint/server/config"

type Config struct {
	config.Config

	MemIAVL MemIAVLConfig `mapstructure:"memiavl"`
}

type MemIAVLConfig struct {
	// Enable defines if the memiavl should be enabled.
	Enable bool `mapstructure:"enable"`
	// ZeroCopy defines if the memiavl should return slices pointing to mmap-ed buffers directly (zero-copy),
	// the zero-copied slices must not be retained beyond current block's execution.
	ZeroCopy bool `mapstructure:"zero-copy"`
	// AsyncCommitBuffer defines the size of asynchronous commit queue, this greatly improve block catching-up
	// performance, -1 means synchronous commit.
	AsyncCommitBuffer int `mapstructure:"async-commit-buffer"`
	// SnapshotKeepRecent defines what old snapshots to keep after new snapshots are taken.
	SnapshotKeepRecent uint32 `mapstructure:"snapshot-keep-recent"`
	// SnapshotInterval defines the block interval the memiavl snapshot is taken, default to 1000.
	SnapshotInterval uint32 `mapstructure:"snapshot-interval"`
}

func DefaultMemIAVLConfig() MemIAVLConfig {
	return MemIAVLConfig{}
}
