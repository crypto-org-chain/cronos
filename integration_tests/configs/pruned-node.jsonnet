local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      pruning: 'everything',
      'state-sync'+: {
        'snapshot-interval': 0,
      },
      store+: {
        // don't enable versiondb, since it don't do pruning right now
        streamers: ['file'],
      },
    },
    genesis+: {
      app_state+: {
        feemarket+: {
          params+: {
            min_gas_multiplier: '0',
          },
        },
      },
    },
  },
}
