local config = import 'default.jsonnet';

config {
  'cronos_777-1'+: {
    'account-prefix': 'crc',
    'coin-type': 60,
    key_name: 'signer1',
    accounts: super.accounts[:std.length(super.accounts) - 1] + [super.accounts[std.length(super.accounts) - 1] {
      coins: super.coins + ',100000000000ibcfee',
    }] + [
      {
        name: 'user' + i,
        coins: '30000000000000000000000basetcro',
      }
      for i in std.range(1, 50)
    ],
    'app-config'+: {
      'index-events': super['index-events'] + ['message.action'],
    },
    genesis+: {
      app_state+: {
        cronos+: {
          params+: {
            max_callback_gas: 50000,
          },
        },
        feemarket+: {
          params+: {
            no_base_fee: true,
            base_fee: '0',
          },
        },
      },
    },
  },
  'chainmain-1': {
    cmd: 'chain-maind',
    'start-flags': '--trace',
    'account-prefix': 'cro',
    'coin-type': 394,
    'app-config': {
      'minimum-gas-prices': '0basecro',
    },
    validators: [
      {
        coins: '2234240000000000000cro',
        staked: '10000000000000cro',
        mnemonic: '${VALIDATOR1_MNEMONIC}',
        base_port: 26800,
      },
      {
        coins: '987870000000000000cro',
        staked: '20000000000000cro',
        mnemonic: '${VALIDATOR2_MNEMONIC}',
        base_port: 26810,
      },
    ],
    accounts: [
      {
        name: 'community',
        coins: '10000000000000cro',
        mnemonic: '${COMMUNITY_MNEMONIC}',
      },
      {
        name: 'relayer',
        coins: '10000000000000cro',
        mnemonic: '${SIGNER1_MNEMONIC}',
      },
      {
        name: 'signer2',
        coins: '10000000000000cro,100000000000ibcfee',
        mnemonic: '${SIGNER2_MNEMONIC}',
      },
    ] + [
      {
        name: 'user' + i,
        coins: '10000000000000cro',
      }
      for i in std.range(1, 50)
    ],
    genesis: {
      app_state: {
        staking: {
          params: {
            unbonding_time: '1814400s',
          },
        },
        gov: {
          voting_params: {
            voting_period: '1814400s',
          },
          deposit_params: {
            max_deposit_period: '1814400s',
            min_deposit: [
              {
                denom: 'basecro',
                amount: '10000000',
              },
            ],
          },
        },
        transfer: {
          params: {
            receive_enabled: true,
            send_enabled: true,
          },
        },
        interchainaccounts: {
          host_genesis_state: {
            params: {
              allow_messages: [
                '/cosmos.bank.v1beta1.MsgSend',
                '/cosmos.staking.v1beta1.MsgDelegate',
                '/ibc.applications.interchain_accounts.host.v1.MsgModuleQuerySafe',
              ],
            },
          },
        },
      },
    },
  },
  relayer: {
    mode: {
      clients: {
        enabled: true,
        refresh: true,
        misbehaviour: false,
      },
      connections: {
        enabled: true,
      },
      channels: {
        enabled: true,
      },
      packets: {
        enabled: true,
        tx_confirmation: true,
      },
    },
    rest: {
      enabled: true,
      host: '127.0.0.1',
      port: 3000,
    },
    chains: [
      {
        id: 'cronos_777-1',
        max_gas: 2500000,
        gas_multiplier: 1.1,
        address_type: {
          derivation: 'ethermint',
          proto_type: {
            pk_type: '/ethermint.crypto.v1.ethsecp256k1.PubKey',
          },
        },
        gas_price: {
          price: 10000000,
          denom: 'basetcro',
        },
        event_source: {
          batch_delay: '5000ms',
        },
        extension_options: [{
          type: 'ethermint_dynamic_fee',
          value: '1000000',
        }],
      },
      {
        id: 'chainmain-1',
        max_gas: 500000,
        gas_price: {
          price: 1000000,
          denom: 'basecro',
        },
        event_source: {
          batch_delay: '5000ms',
        },
      },
    ],
  },
}
