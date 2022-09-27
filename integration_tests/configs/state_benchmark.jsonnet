local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'start-flags': '--trace --streamers file,versiondb',
    'app-config'+: {
      'app-db-backend': 'rocksdb',
      'state-sync'+: {
        'snapshot-interval': 0,
      },
    },
    validators: [
      super.validators[0],
      super.validators[1] {
        'app-config'+: {
          pruning: 'everything',
        },
      },
    ] + super.validators[2:],
    genesis+: {
      consensus_params+: {
        block+: {
          max_gas: '163000000',
        },
      },
      app_state+: {
        feemarket+: {
          params+: {
            no_base_fee: true,
            min_gas_multiplier: '0',
          },
        },
      },
    },
  },
}
