module github.com/crypto-org-chain/cronos/memiavl

go 1.20

require (
	github.com/cosmos/iavl v0.19.5
	github.com/hashicorp/go-multierror v1.1.1
	github.com/ledgerwatch/erigon-lib v0.0.0-20230210071639-db0e7ed11263
	github.com/stretchr/testify v1.8.1
	github.com/tendermint/tm-db v0.6.7
)

require (
	github.com/DataDog/zstd v1.4.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/confio/ics23/go v0.7.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/badger/v2 v2.2007.2 // indirect
	github.com/dgraph-io/ristretto v0.0.3-0.20200630154024-f66de99634de // indirect
	github.com/dgryski/go-farm v0.0.0-20190423205320-6a90982ecee2 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/linxGnu/grocksdb v1.7.10 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7 // indirect
	github.com/tendermint/tendermint v0.34.20 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	golang.org/x/crypto v0.6.0 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	google.golang.org/protobuf v1.28.2-0.20220831092852-f930b1dc76e8 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
	// use informal system fork of tendermint
	github.com/tendermint/tendermint => github.com/informalsystems/tendermint v0.34.26
	// https://github.com/crypto-org-chain/tm-db/tree/release/v0.6.x
	github.com/tendermint/tm-db => github.com/crypto-org-chain/tm-db v0.6.8-0.20230118040049-14dc6b00a5b3
)
