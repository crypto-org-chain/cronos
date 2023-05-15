local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      'app-db-backend': 'rocksdb',
      'minimum-gas-prices': '100000000000basetcro',
      memiavl:: super.memiavl,
      store:: super.store,
      streamers:: super.streamers,
      'iavl-lazy-loading':: super['iavl-lazy-loading'],
    },
    validators: [super.validators[0] {
      'app-config':: super['app-config'],
    }] + super.validators[1:],
    genesis+: {
      app_state+: {
        feemarket+: {
          params+: {
            no_base_fee: true,
          },
        },
      },
    },
  },
}
