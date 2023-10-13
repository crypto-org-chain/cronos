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
        evm+: {
          params+: {
            // emulate the environment on production network
            extra_eips: [
              '2929',
              '2200',
              '1884',
              '1344',
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
