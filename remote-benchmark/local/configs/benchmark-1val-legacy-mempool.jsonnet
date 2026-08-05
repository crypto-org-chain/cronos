// Legacy-mempool variant of benchmark-1val.jsonnet for cronosd binaries that
// predate v1.8.0-alpha's app-mempool feature (no --async-check-tx flag, no
// mempool.type='app'). Keep this in sync with benchmark-1val.jsonnet's
// non-mempool fields by hand - there is no shared base to inherit from.
{
  dotenv: '../../../scripts/.env',
  'cronos_777-1': {
    cmd: 'cronosd',
    'start-flags': '--trace',
    config: {
      db_backend: 'rocksdb',
      event_bus_buffer_capacity: 128,
      rpc: {
        'max_open_connections': 10000,
      },
      mempool: {
        size: 50000,
        recheck: false,
      },
      consensus: {
        timeout_commit: '20ms',
      },
      tx_index: {
        indexer: 'null',
      },
      p2p: {
        libp2p: {
          enabled: true,
        },
      },
    },
    'app-config': {
      chain_id: 'cronos_777-1',
      'app-db-backend': 'rocksdb',
      'minimum-gas-prices': '0basetcro',
      'index-events': ['ethereum_tx.ethereumTxHash'],
      'iavl-lazy-loading': true,
      'json-rpc': {
        address: '127.0.0.1:{EVMRPC_PORT}',
        'ws-address': '127.0.0.1:{EVMRPC_PORT_WS}',
        api: 'eth,net,web3,debug,cronos',
        'feehistory-cap': 100,
        'block-range-cap': 10000,
        'logs-cap': 10000,
        'enable-indexer': false,
      },
      // app-level PriorityNonceMempool (predates v1.8's app-mempool bridge -
      // app/app.go wires this whenever mempool.max-txs is set) - without it
      // PrepareProposal falls back to NoOpMempool, which selects txs in
      // arbitrary gossip-arrival order instead of nonce/priority order.
      mempool: {
        'max-txs': 50000,
      },
      evm: {
        'block-executor': 'block-stm',
        'block-stm-workers': 0,
        'block-stm-pre-estimate': false,
      },
      memiavl: {
        enable: true,
        'zero-copy': true,
        'snapshot-interval': 5,
        'cache-size': 0,
        'async-commit-buffer': 16,
      },
      grpc: {
        'skip-check-header': true,
      },
      cronos: {
        'tx-cache-size': 50000,
      },
    },
    validators: [
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR1_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
    ],
    accounts: [
      {
        name: 'community',
        coins: '1000000000000000000000000000basetcro',
        mnemonic: '${COMMUNITY_MNEMONIC}',
      },
      {
        name: 'signer1',
        coins: '20000000000000000000000basetcro',
        mnemonic: '${SIGNER1_MNEMONIC}',
      },
      {
        name: 'signer2',
        coins: '30000000000000000000000basetcro',
        mnemonic: '${SIGNER2_MNEMONIC}',
      },
    ],
    genesis: {
      consensus: {
        params: {
          block: {
            max_gas: '363000000',
          },
        },
      },
      app_state: {
        evm: {
          params: {
            evm_denom: 'basetcro',
          },
        },
        cronos: {
          params: {
            cronos_admin: '${CRONOS_ADMIN}',
            enable_auto_deployment: true,
            ibc_cro_denom: '${IBC_CRO_DENOM}',
          },
        },
        gov: {
          params: {
            expedited_voting_period: '1s',
            voting_period: '10s',
            max_deposit_period: '10s',
            min_deposit: [
              {
                denom: 'basetcro',
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
        feemarket: {
          params: {
            no_base_fee: false,
            base_fee: '100000000000',
          },
        },
      },
    },
  },
}
