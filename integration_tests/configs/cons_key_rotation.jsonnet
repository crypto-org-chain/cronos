local config = import 'default.jsonnet';

// Power 3:2:1:1. Rotating node2 and node3 together, or stopping node1, leaves
// 5/7 online; with node1 down every commit needs both rotated validators.
config {
  'cronos_777-1'+: {
    validators: [super.validators[0] {
      coins: '3000000000000000000stake,10000000000000000000000basetcro',
      staked: '3000000000000000000stake',
    }, super.validators[1] {
      coins: '2000000000000000000stake,10000000000000000000000basetcro',
      staked: '2000000000000000000stake',
    }, super.validators[2] {
      // liquid stake pays the key rotation fee
      coins: '1000000000000000001stake,10000000000000000000000basetcro',
    }, {
      coins: '1000000000000000001stake,10000000000000000000000basetcro',
      staked: '1000000000000000000stake',
      mnemonic: '${SIGNER3_MNEMONIC}',
      client_config: {
        'broadcast-mode': 'sync',
      },
    }],
    genesis+: {
      app_state+: {
        staking: {
          params: {
            // mirrors the v1.9 upgrade handler
            key_rotation_fee: {
              denom: 'stake',
              amount: '1',
            },
          },
        },
      },
    },
  },
}
