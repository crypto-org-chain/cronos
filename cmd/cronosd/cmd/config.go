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
}

func DefaultPreconferConfig() PreconferConfig {
	return PreconferConfig{
		Enable: true, // Enabled by default for backward compatibility
	}
}

var DefaultPreconferTemplate = `
[preconfer]
# Enable defines if the priority transaction selector should be enabled.
# When enabled, transactions with PRIORITY: prefix in memo will be prioritized.
enable = {{ .Preconfer.Enable }}
`
