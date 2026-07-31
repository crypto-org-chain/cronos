// Same as mempool_app.jsonnet but with a low mempool-ttl-num-blocks and a
// tiny block gas limit, so a test can starve a low-priority tx out of every
// proposal (via a higher-priority flood) and observe TTL eviction without
// waiting through 120 blocks or an unreasonably large flood.
local appmempool = import 'mempool_app.jsonnet';

appmempool {
  'cronos_777-1'+: {
    'app-config'+: {
      cronos+: {
        'mempool-ttl-num-blocks': 2,
      },
    },
    genesis+: {
      consensus+: {
        params+: {
          block+: {
            // ~2 basic 21000-gas transfers per block.
            max_gas: '48000',
          },
        },
      },
    },
  },
}
