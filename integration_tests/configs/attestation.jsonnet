// Configuration for attestation integration test
// Sets up Cronos chain and Attestation Layer chain with Hermes relayer

local config = import 'default.jsonnet';

config {
  // Cronos chain configuration
  'cronos_777-1'+: {
    'config'+: {
      log_level: 'debug',
    },
    'account-prefix': 'crc',
    'coin-type': 60,
    key_name: 'signer1',
    'app-config'+: {
      'minimum-gas-prices': '0basecro',
      'index-events': super['index-events'] + ['message.action'],
    },
    // Override accounts to use basecro instead of basetcro
    accounts: [
      {
        name: 'community',
        coins: '10000000000000000000000basecro',
        mnemonic: '${COMMUNITY_MNEMONIC}',
      },
      {
        name: 'signer1',
        coins: '20000000000000000000000basecro',
        mnemonic: '${SIGNER1_MNEMONIC}',
      },
      {
        name: 'signer2',
        coins: '30000000000000000000000basecro,100000000000ibcfee',
        mnemonic: '${SIGNER2_MNEMONIC}',
      },
    ],
    genesis+: {
      app_state+: {
        evm+: {
          params+: {
            evm_denom: 'basecro',
          },
        },
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
        // Attestation module configuration
        attestation: {
          params: {
            attestation_enabled: true,
            attestation_interval: 10,  // Send attestation every 10 blocks
            packet_timeout_timestamp: 600000000000,  // 10 minutes
          },
          v2_client_id: '07-tendermint-0',  // Set after IBC client creation (client ID not known at genesis)
        },
      },
    },
  },
  // Attestation Layer chain configuration
  'attestation-1': {
    cmd: 'cronos-attestad',
    'start-flags': '--trace',
    'account-prefix': 'cosmos',
    'coin-type': 118,
    config: {
      log_level: 'debug',
      consensus: {
        'timeout-commit': '1s',
      },
    },
    'app-config': {
      'minimum-gas-prices': '0stake',
    },
    validators: [
      {
        coins: '2234240000000000000stake',
        staked: '10000000000000stake',
        mnemonic: '${VALIDATOR1_MNEMONIC}',
        base_port: 26800,
      },
      {
        coins: '987870000000000000stake',
        staked: '20000000000000stake',
        mnemonic: '${VALIDATOR2_MNEMONIC}',
        base_port: 26810,
      },
    ],
    accounts: [
      {
        name: 'community',
        coins: '10000000000000stake',
        mnemonic: '${COMMUNITY_MNEMONIC}',
      },
      {
        name: 'relayer',
        coins: '10000000000000stake',
        mnemonic: '${SIGNER1_MNEMONIC}',
      },
      {
        name: 'signer2',
        coins: '10000000000000stake,100000000000ibcfee',
        mnemonic: '${SIGNER2_MNEMONIC}',
      },
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
                denom: 'stake',
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
    global: {
      log_level: 'debug',
    },
    mode: {
      clients: {
        enabled: true,
        refresh: true,
        misbehaviour: false,
      },
      connections: {
        enabled: false,  // V2 doesn't use connections
      },
      channels: {
        enabled: false,  // V2 doesn't use channels
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
    telemetry: {
      enabled: true,
      host: '127.0.0.1',
      port: 3001,
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
          denom: 'basecro',
        },
        event_source: {
          batch_delay: '1000ms',
        },
        extension_options: [{
          type: 'ethermint_dynamic_fee',
          value: '1000000',
        }],
      },
      {
        id: 'attestation-1',
        max_gas: 500000,
        gas_price: {
          price: 1000,
          denom: 'stake',
        },
        event_source: {
          batch_delay: '1000ms',
        },
      },
    ],
  },
}

