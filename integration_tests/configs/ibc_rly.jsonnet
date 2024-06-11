local ibc = import 'ibc.jsonnet';

ibc {
  'chainmain-1'+: {
    validators: [
      {
        coins: '987870000000000000cro',
        staked: '20000000000000cro',
        mnemonic: '${VALIDATOR' + i + '_MNEMONIC}',
        base_port: 26800 + i * 10,
      }
      for i in std.range(1, 2)
    ],
  },
}
