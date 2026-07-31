// Same as mempool_app.jsonnet but with a low mempool.max-txs, so a test can
// reach saturation without submitting hundreds of txs.
local appmempool = import 'mempool_app.jsonnet';

appmempool {
  'cronos_777-1'+: {
    'app-config'+: {
      mempool+: {
        'max-txs': 5,
      },
    },
  },
}
