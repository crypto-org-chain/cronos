local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    config+: {
      mempool: {
        recheck: false,
      },
    },
    validators+: [{
      coins: '1000000000000000000stake,10000000000000000000000basetcro',
      staked: '1000000000000000000stake',
      client_config: {
        'broadcast-mode': 'sync',
      },
    }, {
      coins: '1000000000000000000stake,10000000000000000000000basetcro',
      staked: '1000000000000000000stake',
      client_config: {
        'broadcast-mode': 'sync',
      },
    }],
  },
}
