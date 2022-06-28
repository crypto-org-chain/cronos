{
  validator(mnemonic):: {
    coins: '1000000000000000000stake,10000000000000000000000basetcro',
    staked: '1000000000000000000stake',
    mnemonic: mnemonic,
  },
  validators(mnemonics):: [self.validator(mnemonic) for mnemonic in mnemonics],
  accounts(objs):: [obj for obj in objs],
}
