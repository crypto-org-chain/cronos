local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'app-config'+: {
      'minimum-gas-prices': '100000000000basetcro',
      store:: super.store,
    },
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
            initial_base_fee: super.base_fee,
          },
        },
      },
    },
  },
}
