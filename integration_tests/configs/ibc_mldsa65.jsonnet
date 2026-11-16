local ibc = import 'ibc.jsonnet';
local rotation = import 'cons_key_rotation.jsonnet';
local cronos = ibc['cronos_777-1'];
local relayer_chain = ibc.relayer.chains[0];

// The pinned chain-main's cometbft predates ML-DSA-65 support (v0.38.26), so
// the counterparty is a second cronos chain.
{
  dotenv: ibc.dotenv,
  'cronos_777-1': cronos {
    validators: rotation['cronos_777-1'].validators,
    genesis+: {
      app_state+: {
        staking: rotation['cronos_777-1'].genesis.app_state.staking,
      },
    },
  },
  'cronos_778-1': cronos {
    'app-config'+: {
      chain_id: 'cronos_778-1',
    },
    validators: [
      cronos.validators[i] { base_port: 26800 + i * 10 }
      for i in [0, 1]
    ],
  },
  relayer: ibc.relayer {
    chains: [relayer_chain, relayer_chain { id: 'cronos_778-1' }],
  },
}
