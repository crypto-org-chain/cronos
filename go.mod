module github.com/crypto-org-chain/cronos

go 1.16

require (
	github.com/armon/go-metrics v0.3.9
	github.com/cosmos/cosmos-sdk v0.44.0
	github.com/cosmos/ibc-go v1.1.0
	github.com/ethereum/go-ethereum v1.10.3
	github.com/gogo/protobuf v1.3.3
	github.com/golang/protobuf v1.5.2
	github.com/gorilla/mux v1.8.0
	github.com/grpc-ecosystem/grpc-gateway v1.16.0
	github.com/peggyjv/gravity-bridge/module v0.2.0
	github.com/spf13/cast v1.4.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/tendermint/spm v0.0.0-20210524110815-6d7452d2dc4a
	github.com/tendermint/tendermint v0.34.12
	github.com/tendermint/tm-db v0.6.4
	github.com/tharsis/ethermint v0.4.2-0.20210905110306-26c5eabb1893
	google.golang.org/genproto v0.0.0-20210903162649-d08c68adba83
	google.golang.org/grpc v1.40.0
	gopkg.in/yaml.v2 v2.4.0
)

replace google.golang.org/grpc => google.golang.org/grpc v1.33.2

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1

// TODO: fix keyring upstream
replace github.com/99designs/keyring => github.com/crypto-org-chain/keyring v1.1.6-fixes

// TODO: remove when middleware will be implemented
replace github.com/cosmos/ibc-go => github.com/crypto-org-chain/ibc-go v1.0.1-hooks

// FIXME: update after PR merged: https://github.com/PeggyJV/gravity-bridge/pull/182
replace github.com/peggyjv/gravity-bridge/module => github.com/yihuang/gravity-bridge/module v0.1.22-0.20210908191543-71021d1bfff2
