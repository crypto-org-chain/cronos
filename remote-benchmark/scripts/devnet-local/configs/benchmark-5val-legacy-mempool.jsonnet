// Legacy-mempool variant of benchmark-5val.jsonnet for cronosd binaries that
// predate v1.8.0-alpha's app-mempool feature. Keep in sync with
// benchmark-5val.jsonnet's non-mempool fields by hand - there is no shared
// base to inherit from.
{
  dotenv: '../../../../scripts/.env',
  'cronos_777-1': {
    cmd: 'cronosd',
    'start-flags': '--trace',
    config: {
      db_backend: 'goleveldb',
      event_bus_buffer_capacity: 128,
      rpc: {
        'max_open_connections': 10000,
      },
      mempool: {
        size: 2000,
        recheck: true,
      },
      consensus: {
        // Phase 1 baseline: mainnet-realistic timeouts (tuning-doc Phase 1 /
        // validator-config.toml), not the aggressive Phase-2 tuning values.
        timeout_propose: '3s',
        timeout_propose_delta: '500ms',
        timeout_prevote: '1s',
        timeout_prevote_delta: '500ms',
        timeout_precommit: '1s',
        timeout_precommit_delta: '500ms',
        timeout_commit: '200ms',
        create_empty_blocks_interval: '5s',
        peer_gossip_sleep_duration: '100ms',
        peer_query_maj23_sleep_duration: '2s',
        skip_timeout_commit: false,
      },
      tx_index: {
        indexer: 'null',
      },
      p2p: {
        libp2p: {
          enabled: false,
        },
        // Default (0s) means unbounded exponential backoff between persistent-
        // peer redials. All 5 validators start concurrently, so node A's very
        // first dial to node B often loses the race against B's listener
        // coming up and gets "connection refused" - with no cap, the backoff
        // after that then balloons to minutes, and the p2p mesh gets stuck
        // below the 4-peer quorum needed to ever produce block 1.
        persistent_peers_max_dial_period: '3s',
      },
    },
    'app-config': {
      chain_id: 'cronos_777-1',
      'app-db-backend': 'goleveldb',
      'minimum-gas-prices': '0basetcro',
      'index-events': ['ethereum_tx.ethereumTxHash'],
      'async-check-tx': true,
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
      mempool: {
        'max-txs': -1,
      },
      evm: {
        'block-executor': 'block-stm',
        'block-stm-workers': 0,
        'block-stm-pre-estimate': false,
      },
      memiavl: {
        enable: false,
      },
      grpc: {
        'skip-check-header': true,
      },
      cronos: {
        'tx-cache-size': 100000,
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
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR2_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR3_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR4_MNEMONIC}',
        client_config: {
          'broadcast-mode': 'sync',
        },
      },
      {
        coins: '1000000000000000000stake,10000000000000000000000basetcro',
        staked: '1000000000000000000stake',
        mnemonic: '${VALIDATOR5_MNEMONIC}',
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
            max_gas: '105000000',
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
            // baseFee escalation would permanently gate-skip fixed-fee-cap txs
            // during this benchmark's unthrottled burst (no pacing between
            // sends, unlike the wiki tooling's send_interval); disable it so
            // the run measures raw throughput instead of fee-market dynamics.
            no_base_fee: true,
            base_fee: '100000000000',
          },
        },
      },
    },
  },
}
