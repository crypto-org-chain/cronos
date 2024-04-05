local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      'app-db-backend': 'rocksdb',
      'minimum-gas-prices': '100000000000basetcro',
      'iavl-lazy-loading':: super['iavl-lazy-loading'],
    },
    genesis+: {
      app_state+: {
        bank+: {
          params: {
            send_enabled: [
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
        },
        feemarket+: {
          params+: {
            no_base_fee: false,
            base_fee:: super.base_fee,
          },
        },
        gov: {
          voting_params: {
            voting_period: '10s',
          },
          deposit_params: {
            max_deposit_period: '10s',
            min_deposit: [
              {
                denom: 'basetcro',
                amount: '1',
              },
            ],
          },
        },
      },
    },
  },
}
