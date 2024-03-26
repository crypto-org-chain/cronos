local ibc = import 'ibc_rly.jsonnet';

ibc {
  relayer+: {
    chains: [super.chains[0] {
      precompiled_contract_address: '0x6F1805D56bF05b7be10857F376A5b1c160C8f72C',
      json_rpc_address: 'http://127.0.0.1:26701',
    }] + super.chains[1:],
  },
}
