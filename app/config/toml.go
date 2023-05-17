package config

// DefaultConfigTemplate defines the configuration template for the EVM RPC configuration
const DefaultConfigTemplate = `
###############################################################################
###                             Cronos Configuration                        ###
###############################################################################

[memiavl]

# Enable defines if the memiavl should be enabled.
enable = {{ .MemIAVL.Enable }}

# ZeroCopy defines if the memiavl should return slices pointing to mmap-ed buffers directly (zero-copy),
# the zero-copied slices must not be retained beyond current block's execution.
zero-copy = {{ .MemIAVL.ZeroCopy }}

# AsyncCommitBuffer defines the size of asynchronous commit queue, this greatly improve block catching-up
# performance, -1 means synchronous commit.
async-commit-buffer = {{ .MemIAVL.AsyncCommitBuffer }}
`
