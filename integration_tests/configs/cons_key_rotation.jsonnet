local config = import 'default.jsonnet';

// 4 equal validators, so the chain keeps 3/4 of the power while the rotated node restarts.
config {
  'cronos_777-1'+: {
    validators+: [{
      // liquid stake pays the key rotation fee
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
