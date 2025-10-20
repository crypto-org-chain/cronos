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

// SequencerKeyConfig represents a single sequencer public key configuration
type SequencerKeyConfig struct {
	ID     string `mapstructure:"id"`
	Type   string `mapstructure:"type"`
	PubKey string `mapstructure:"pubkey"`
}

type SequencerConfig struct {
	// Enable defines if the sequencer-based transaction ordering should be enabled.
	Enable bool `mapstructure:"enable"`
	// Keys is a list of sequencer public keys for signature verification.
	Keys []SequencerKeyConfig `mapstructure:"keys"`
}

func DefaultSequencerConfig() SequencerConfig {
	return SequencerConfig{
		Enable: false,                  // Disabled by default
		Keys:   []SequencerKeyConfig{}, // Empty by default
	}
}

var DefaultSequencerTemplate = `
###############################################################################
###                        Sequencer Configuration                          ###
###############################################################################

[sequencer]
# Enable defines if the sequencer-based transaction ordering should be enabled.
# When enabled, the ExecutionBook will manage transactions from registered sequencers.
enable = {{ .Sequencer.Enable }}

# Sequencer public keys for signature verification.
# Each sequencer must be registered with its public key before it can submit transactions.
# 
# Configuration format:
# [[sequencer.keys]]
# id = "sequencer1"
# type = "ed25519"
# pubkey = "hex_encoded_public_key"
#
# Supported key types:
# - ed25519: 32-byte (64 hex chars) Ed25519 public keys (recommended for performance)
# - ecdsa: 33-byte (66 hex chars) compressed or 65-byte (130 hex chars) uncompressed ECDSA keys
#   (Ethereum-compatible, also accepts "eth_secp256k1" as type name)
#
# Example configuration:
# [[sequencer.keys]]
# id = "primary-sequencer"
# type = "ed25519"
# pubkey = "a1b2c3d4e5f6789012345678901234567890123456789012345678901234abcd"
#
# [[sequencer.keys]]
# id = "eth-sequencer"
# type = "ecdsa"
# pubkey = "02a1b2c3d4e5f6789012345678901234567890123456789012345678901234ab"
{{ range .Sequencer.Keys }}
[[sequencer.keys]]
id = "{{ .ID }}"
type = "{{ .Type }}"
pubkey = "{{ .PubKey }}"
{{ end }}
`
