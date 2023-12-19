local ibc = import 'ibc_rly.jsonnet';

ibc {
  relayer+: {
    chains: [super.chains[0] {
      precompiled_contract_address: '0x0000000000000000000000000000000000000065',
    }] + super.chains[1:],
  },
}
