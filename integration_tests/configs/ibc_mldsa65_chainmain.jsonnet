local rotation = import 'cons_key_rotation.jsonnet';
local ibc = import 'ibc.jsonnet';

// chain-main needs cometbft v0.38.26+ to verify ML-DSA-65 Cronos headers.
ibc {
  'cronos_777-1'+: {
    validators: rotation['cronos_777-1'].validators,
    genesis+: {
      app_state+: {
        staking: rotation['cronos_777-1'].genesis.app_state.staking,
      },
    },
  },
}
