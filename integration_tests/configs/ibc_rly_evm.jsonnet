local ibc = import 'ibc_rly.jsonnet';

ibc {
  relayer+: {
    chains: [super.chains[0] {
      precompiled_contract_address: '0x0000000000000000000000000000000000000065',
      extension_options: [{
        type: 'ethermint_dynamic_fee',
        value: '10000000000000',  //greater than minimum global fee
      }],
    }] + super.chains[1:],
  },
}
