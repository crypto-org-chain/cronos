local ibc = import 'ibc_rly.jsonnet';

ibc {
  relayer+: {
    chains: [super.chains[0] {
      precompiled_contract_address: '0x0000000000000000000000000000000000000065',
      json_rpc_address: 'http://127.0.0.1:26701',
    }] + super.chains[1:],
  },
}
