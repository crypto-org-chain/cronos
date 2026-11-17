// Same as mempool_app.jsonnet but with cronos.disable-tx-replacement=true.
// The ante gets no pool to look up and the pool rule refuses replacement, so a
// same-nonce tx fails the nonce check or, after a Commit reset, the pool rule.
local appmempool = import 'mempool_app.jsonnet';

appmempool {
  'cronos_777-1'+: {
    'app-config'+: {
      cronos+: {
        'disable-tx-replacement': true,
      },
    },
  },
}
