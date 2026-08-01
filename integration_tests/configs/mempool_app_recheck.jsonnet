// Companion to mempool_app_no_recheck.jsonnet: same tiny block gas limit, but
// recheck left on, so a test can show the sweep actually evicting the tx that
// the recheck=false run leaves pending.
local appmempool = import 'mempool_app.jsonnet';

appmempool {
  'cronos_777-1'+: {
    config+: {
      mempool+: {
        recheck: true,
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
