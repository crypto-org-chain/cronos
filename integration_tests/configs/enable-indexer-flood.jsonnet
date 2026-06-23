// enable-indexer base + CometBFT flood mempool. The eth "pending" filter reads
// CometBFT UnconfirmedTxs, which is empty under mempool.type=app.
local config = import 'enable-indexer.jsonnet';

config {
  'cronos_777-1'+: {
    config+: {
      mempool: { version: 'v1' },
    },
  },
}
