local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      'app-db-backend': 'rocksdb',
      'minimum-gas-prices': '100000000000basetcro',
      'iavl-lazy-loading':: super['iavl-lazy-loading'],
    },
    genesis+: {
      consensus_params+: {
        block+: {
          max_gas: '60000000',
        },
      },
      app_state+: {
        bank+: {
          send_enabled+: [
            {
              denom: 'stake',
              enabled: true,
            },
            {
              denom: 'basetcro',
              enabled: false,
            },
          ],
        },
        feemarket+: {
          params+: {
            no_base_fee: false,
            base_fee:: super.base_fee,
          },
        },
      },
    },
  },
}
