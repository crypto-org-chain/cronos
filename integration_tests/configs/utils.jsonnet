{
  validator(mnemonic):: {
    coins: '1000000000000000000stake,10000000000000000000000basetcro',
    staked: '1000000000000000000stake',
    mnemonic: mnemonic,
  },
  validator_with_timeout(mnemonic):: self.validator(mnemonic) {
    config: {
      consensus: {
        timeout_commit: '15s',
      },
    },
  },
  validators(mnemonics):: [self.validator(mnemonic) for mnemonic in mnemonics],
  validators_with_timeout(mnemonics):: [self.validator_with_timeout(mnemonic) for mnemonic in mnemonics],
  accounts(objs):: [obj for obj in objs],
}
