// Same as mempool_app.jsonnet but with recheck disabled and a tiny block gas
// limit, so a test can observe that a tx made stale by an earlier same-account
// tx is never re-validated and evicted, unlike the recheck=true default.
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
