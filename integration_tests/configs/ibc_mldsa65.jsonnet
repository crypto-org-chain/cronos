local ibc = import 'ibc.jsonnet';
local rotated = import 'ibc_mldsa65_chainmain.jsonnet';
local cronos = ibc['cronos_777-1'];
local relayer_chain = ibc.relayer.chains[0];

// Same rotated chain as the chain-main case, with a second cronos chain as the
// counterparty to cover Cronos's own light client.
{
  dotenv: ibc.dotenv,
  'cronos_777-1': rotated['cronos_777-1'],
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
