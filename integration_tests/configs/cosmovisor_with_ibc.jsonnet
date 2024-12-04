local ibc = import 'ibc.jsonnet';

ibc {
  'cronos_777-1'+: {
    'app-config'+: {
      'app-db-backend': 'rocksdb',
      'iavl-lazy-loading':: super['iavl-lazy-loading'],
    },
    validators: [super.validators[0] {
      'app-config'+: {
        store: {
          streamers: ['versiondb'],
        },
      },
    }] + super.validators[1:],
    genesis+: {
      consensus_params: {
        block: {
          max_bytes: '1048576',
          max_gas: '81500000',
        },
      },
      app_state+: {
        gov+: {
          params+: {
            expedited_voting_period:: super['expedited_voting_period'],
          },
        },
      },
    },
  },
}
