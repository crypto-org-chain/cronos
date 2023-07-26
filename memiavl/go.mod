module github.com/crypto-org-chain/cronos/memiavl

go 1.20

require (
	cosmossdk.io/errors v1.0.0-beta.7
	github.com/cometbft/cometbft v0.37.2
	github.com/cometbft/cometbft-db v0.7.0
	github.com/confio/ics23/go v0.9.0
	github.com/cosmos/cosmos-sdk v0.47.3
	github.com/cosmos/gogoproto v1.4.10
	github.com/cosmos/iavl v0.20.0
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d
	github.com/ledgerwatch/erigon-lib v0.0.0-20230210071639-db0e7ed11263
	github.com/stretchr/testify v1.8.3
	github.com/tidwall/btree v1.6.0
	github.com/tidwall/gjson v1.10.2
	github.com/tidwall/wal v1.1.7
	github.com/zbiljic/go-filelock v0.0.0-20170914061330-1dbf7103ab7d
	golang.org/x/exp v0.0.0-20230515195305-f3d0a9c9a5cc
	golang.org/x/sync v0.1.0
)

require (
	github.com/DataDog/zstd v1.4.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/btcsuite/btcd/btcec/v2 v2.3.2 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cockroachdb/errors v1.8.1 // indirect
	github.com/cockroachdb/logtags v0.0.0-20190617123548-eb05cc24525f // indirect
	github.com/cockroachdb/pebble v0.0.0-20230315005856-dcb60b9212f9 // indirect
	github.com/cockroachdb/redact v1.0.8 // indirect
	github.com/cockroachdb/sentry-go v0.6.1-cockroachdb.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.1.0 // indirect
	github.com/dgraph-io/badger/v2 v2.2007.4 // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.1.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/jmhodges/levigo v1.0.0 // indirect
	github.com/klauspost/compress v1.16.3 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/linxGnu/grocksdb v1.8.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/petermattis/goid v0.0.0-20230317030725-371a4b8eda08 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.42.0 // indirect
	github.com/prometheus/procfs v0.9.0 // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/sasha-s/go-deadlock v0.3.1 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/tendermint/go-amino v0.16.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/tinylru v1.1.0 // indirect
	go.etcd.io/bbolt v1.3.7 // indirect
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/grpc v1.55.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace (
	// use cometbft
	github.com/cometbft/cometbft => github.com/cometbft/cometbft v0.37.2
	github.com/cometbft/cometbft-db => github.com/crypto-org-chain/cometbft-db v0.0.0-20230412133340-ac70df4b45f6
	github.com/cosmos/cosmos-sdk => github.com/mmsqe/cosmos-sdk v0.47.0-rc1.0.20230726065139-713089f44416
	github.com/cosmos/iavl => github.com/mmsqe/iavl v0.19.2-0.20230629030159-de567266d55b
)
