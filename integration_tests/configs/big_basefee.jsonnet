local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    // provide gas prices for genesis txs
    validators: [validator {
      gas_prices: '1000000000000000000basetcro',
      coins: '1000000000000000000stake,600000000000000000000000basetcro',
    } for validator in super.validators],
    genesis+: {
      app_state+: {
        feemarket+: {
          params+: {
            base_fee_change_denominator: '3',
            elasticity_multiplier: '4',
            // 100 cro
            base_fee: '100000000000000000000',
            // 1 cro
            min_gas_price: '1000000000000000000',
          },
        },
      },
    },
  },
}
