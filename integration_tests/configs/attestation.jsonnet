// Configuration for attestation integration test
// Sets up Cronos chain and Attestation Layer chain with Hermes relayer

local config = import 'default.jsonnet';

config {
  // Cronos chain configuration
  'cronos_777-1'+: {
    cmd: 'cronosd',
    'app-config'+: {
      'minimum-gas-prices': '0basecro',
      'index-events': ['ethereum_tx.ethereumTxHash'],
      'json-rpc'+: {
        address: '127.0.0.1:{EVMRPC_PORT}',
        'ws-address': '127.0.0.1:{EVMRPC_PORT_WS}',
        api: 'eth,net,web3,debug,cronos',
        'feehistory-cap': 100,
        'block-range-cap': 10000,
        'logs-cap': 10000,
      },
    },
    config+: {
      consensus+: {
        'timeout-commit': '1s',
      },
    },
    'account-prefix': 'cro',
    'coin-type': 394,
    genesis+: {
      app_state+: {
        feemarket+: {
          params+: {
            no_base_fee: false,
            base_fee: '100000000000',
          },
        },
        gov: {
          voting_params: {
            voting_period: '10s',
          },
          deposit_params: {
            max_deposit_period: '10s',
            min_deposit: [
              {
                denom: 'basecro',
                amount: '1',
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
        // Attestation module configuration
        attestation: {
          params: {
            attestation_enabled: true,
            attestation_interval: '10',  // Send attestation every 10 blocks
            packet_timeout_timestamp: '600000000000',  // 10 minutes
          },
        },
      },
    },
    validators: [
      {
        coins: '2234567000000000000000000basecro',
        staked: '10000000000000000000basecro',
        mnemonic: '${VALIDATOR1_MNEMONIC}',
        base_port: 26650,
      },
      {
        coins: '987870000000000000000000basecro',
        staked: '20000000000000000000basecro',
        mnemonic: '${VALIDATOR2_MNEMONIC}',
        base_port: 26660,
      },
    ],
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
        coins: '30000000000000000000000basecro',
        mnemonic: '${SIGNER2_MNEMONIC}',
      },
    ],
    genesis_opts: {
      'app_state.evm.params.evm_denom': 'basecro',
    },
  },
  
  // Attestation Layer chain configuration
  'attestation-1': {
    cmd: 'cronos-attestad',
    'start-flags': '--trace',
    'key-name': 'validator',
    'keyring-backend': 'test',
    'account-prefix': 'cro',
    'coin-type': 394,
    config: {
      consensus: {
        'timeout-commit': '1s',
      },
    },
    'app-config': {
      'chain_id': 'attestation-1',
      'api': {
        enable: true,
      },
      'minimum-gas-prices': '0stake',
      'iavl-lazy-loading': true,
    },
    validators: [
      {
        coins: '1000000000000stake',
        staked: '100000000stake',
        mnemonic: '${ATTESTA_VALIDATOR1_MNEMONIC}',
        base_port: 27650,
      },
      {
        coins: '1000000000000stake',
        staked: '100000000stake',
        mnemonic: '${ATTESTA_VALIDATOR2_MNEMONIC}',
        base_port: 27660,
      },
    ],
    accounts: [
      {
        name: 'relayer',
        coins: '10000000000stake',
        mnemonic: '${ATTESTA_RELAYER_MNEMONIC}',
      },
    ],
    genesis+: {
      app_state+: {
        transfer: {
          params: {
            receive_enabled: true,
            send_enabled: true,
          },
        },
        cronosattesta: {
          params: {
            // Attestation layer specific params
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
          mode: 'push',
          url: 'ws://127.0.0.1:26657/websocket',
          batch_delay: '500ms',
        },
      },
      {
        id: 'attestation-1',
        max_gas: 500000,
        gas_price: {
          price: 1000,
          denom: 'stake',
        },
        event_source: {
          mode: 'push',
          url: 'ws://127.0.0.1:27657/websocket',
          batch_delay: '500ms',
        },
      },
    ],
  },
}

