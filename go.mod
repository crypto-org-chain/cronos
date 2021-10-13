module github.com/crypto-org-chain/cronos

go 1.16

require (
	github.com/armon/go-metrics v0.3.9
	github.com/cosmos/cosmos-sdk v0.44.1
	github.com/cosmos/ibc-go v1.2.1
	github.com/ethereum/go-ethereum v1.10.3
	github.com/gogo/protobuf v1.3.3
	github.com/golang/protobuf v1.5.2
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/rakyll/statik v0.1.7
	github.com/spf13/cast v1.4.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/tendermint/tendermint v0.34.13
	github.com/tendermint/tm-db v0.6.4
	github.com/tharsis/ethermint v0.7.1
	google.golang.org/genproto v0.0.0-20210909211513-a8c4777a87af
	google.golang.org/grpc v1.41.0
	gopkg.in/yaml.v2 v2.4.0
)

replace google.golang.org/grpc => google.golang.org/grpc v1.33.2

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

// TODO: fix keyring upstream
replace github.com/99designs/keyring => github.com/crypto-org-chain/keyring v1.1.6-fixes

// TODO: remove when middleware will be implemented
replace github.com/cosmos/ibc-go => github.com/crypto-org-chain/ibc-go v1.2.1-hooks

replace github.com/cosmos/iavl => github.com/cosmos/iavl v0.17.1

replace github.com/ethereum/go-ethereum => github.com/crypto-org-chain/go-ethereum v1.10.3-patched
