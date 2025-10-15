package cmd

type VersionDBConfig struct {
	// Enable defines if the versiondb should be enabled.
	Enable bool `mapstructure:"enable"`
}

func DefaultVersionDBConfig() VersionDBConfig {
	return VersionDBConfig{
		Enable: false,
	}
}

var DefaultVersionDBTemplate = `
[versiondb]
# Enable defines if the versiondb should be enabled.
enable = {{ .VersionDB.Enable }}
`

type PreconferConfig struct {
	// Enable defines if the priority tx selector (preconfirmation) should be enabled.
	Enable bool `mapstructure:"enable"`
	// ValidatorAddress is the validator address for signing preconfirmations (optional).
	ValidatorAddress string `mapstructure:"validator_address"`
	// PreconfirmTimeout is the duration before a preconfirmation expires (default: "30s").
	// Accepts Go duration format: "10s", "1m", "90s", "1m30s", etc.
	PreconfirmTimeout string `mapstructure:"preconfirm_timeout"`
	// Whitelist defines the list of Ethereum addresses (0x...) allowed to boost transaction priority.
	// If empty, all addresses are allowed to use priority boosting.
	// If non-empty, only listed addresses can boost priority.
	Whitelist []string `mapstructure:"whitelist"`
}

func DefaultPreconferConfig() PreconferConfig {
	return PreconferConfig{
		Enable:            true,       // Enabled by default for backward compatibility
		ValidatorAddress:  "",         // Empty by default, optional
		PreconfirmTimeout: "30s",      // Default 30 seconds
		Whitelist:         []string{}, // Empty by default, allows all addresses
	}
}

var DefaultPreconferTemplate = `
[preconfer]
# Enable defines if the priority transaction selector should be enabled.
# When enabled, transactions with PRIORITY: prefix in memo will be prioritized.
enable = {{ .Preconfer.Enable }}

# Validator address for signing preconfirmations (optional).
# If not set, preconfirmations will still be created but unsigned.
# Format: Bech32 validator address (e.g., cronosvaloper1...)
validator_address = "{{ .Preconfer.ValidatorAddress }}"

# Preconfirmation timeout duration (default: "30s").
# Time before a preconfirmation expires.
# Accepts Go duration format: "10s", "1m", "90s", "1m30s", "2m", etc.
# Valid time units: ns, us (µs), ms, s, m, h
preconfirm_timeout = "{{ .Preconfer.PreconfirmTimeout }}"

# Whitelist defines the list of Ethereum addresses (0x...) allowed to boost transaction priority.
# If empty (default), all addresses are allowed to use priority boosting.
# If non-empty, only listed addresses can boost priority.
# Example: whitelist = ["0x1234567890123456789012345678901234567890", "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"]
whitelist = [{{ range $i, $addr := .Preconfer.Whitelist }}{{ if $i }}, {{ end }}"{{ $addr }}"{{ end }}]
`
