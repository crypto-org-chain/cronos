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
	// Whitelist defines the list of Ethereum addresses (0x...) allowed to boost transaction priority.
	// If empty, all addresses are allowed to use priority boosting.
	// If non-empty, only listed addresses can boost priority.
	Whitelist []string `mapstructure:"whitelist"`
}

func DefaultPreconferConfig() PreconferConfig {
	return PreconferConfig{
		Enable:    true,       // Enabled by default for backward compatibility
		Whitelist: []string{}, // Empty by default, allows all addresses
	}
}

var DefaultPreconferTemplate = `
[preconfer]
# Enable defines if the priority transaction selector should be enabled.
# When enabled, transactions with PRIORITY: prefix in memo will be prioritized.
enable = {{ .Preconfer.Enable }}

# Whitelist defines the list of Ethereum addresses (0x...) allowed to boost transaction priority.
# If empty (default), all addresses are allowed to use priority boosting.
# If non-empty, only listed addresses can boost priority.
# Example: whitelist = ["0x1234567890123456789012345678901234567890", "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"]
whitelist = [{{ range $i, $addr := .Preconfer.Whitelist }}{{ if $i }}, {{ end }}"{{ $addr }}"{{ end }}]
`
