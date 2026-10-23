// Same as mempool_app.jsonnet but with cronos.disable-tx-replacement=true.
// The ante gets no pool to look up: same-nonce replacements fail at the
// nonce check (ErrInvalidSequence) before reaching PriorityNonceMempool.Insert.
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
