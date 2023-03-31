module github.com/crypto-org-chain/cronos/memiavl

go 1.20

require (
	github.com/cosmos/iavl v0.19.5
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d
	github.com/ledgerwatch/erigon-lib v0.0.0-20230210071639-db0e7ed11263
	github.com/stretchr/testify v1.8.1
	github.com/tendermint/tm-db v0.6.7
	github.com/tidwall/btree v1.5.0
)

require (
	github.com/VictoriaMetrics/metrics v1.23.1 // indirect
	github.com/c2h5oh/datasize v0.0.0-20220606134207-859f65c6625b // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/confio/ics23/go v0.7.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/badger/v2 v2.2007.4 // indirect
	github.com/dgraph-io/ristretto v0.0.3-0.20200630154024-f66de99634de // indirect
	github.com/dgryski/go-farm v0.0.0-20190423205320-6a90982ecee2 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/klauspost/compress v1.15.11 // indirect
	github.com/ledgerwatch/log/v3 v3.7.0 // indirect
	github.com/linxGnu/grocksdb v1.7.10 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7 // indirect
	github.com/tendermint/tendermint v0.34.20 // indirect
	github.com/tidwall/gjson v1.10.2 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/tinylru v1.1.0 // indirect
	github.com/tidwall/wal v1.1.7 // indirect
	github.com/torquem-ch/mdbx-go v0.27.5 // indirect
	github.com/valyala/fastrand v1.1.0 // indirect
	github.com/valyala/histogram v1.2.0 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/crypto v0.6.0 // indirect
	golang.org/x/exp v0.0.0-20230206171751-46f607a40771 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	google.golang.org/protobuf v1.28.2-0.20220831092852-f930b1dc76e8 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
	// use cometbft
	github.com/tendermint/tendermint => github.com/cometbft/cometbft v0.34.27
	// https://github.com/crypto-org-chain/tm-db/tree/release/v0.6.x
	github.com/tendermint/tm-db => github.com/crypto-org-chain/tm-db v0.6.8-0.20230118040049-14dc6b00a5b3
)
