// Same as mempool_app.jsonnet but with recheck disabled and a tiny block gas
// limit. Full blocks keep the base fee climbing, so a tx priced just above the
// base fee it was admitted against is left underpriced within a block or two -
// and with recheck off it is never re-validated or evicted.
local appmempool = import 'mempool_app.jsonnet';

appmempool {
  'cronos_777-1'+: {
    config+: {
      mempool+: {
        recheck: false,
      },
    },
    genesis+: {
      consensus+: {
        params+: {
          block+: {
            // Only one basic 21000-gas transfer per block.
            max_gas: '30000',
          },
        },
      },
    },
  },
}
