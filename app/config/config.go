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
	// AsyncCommit defines if the memiavl should commit asynchronously, this greatly improve block catching-up performance.
	AsyncCommit bool `mapstructure:"async-commit"`
}

func DefaultMemIAVLConfig() MemIAVLConfig {
	return MemIAVLConfig{}
}
