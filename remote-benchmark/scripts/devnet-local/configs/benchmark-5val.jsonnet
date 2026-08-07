// 5-validator devnet config matching the "5 Validators" setup described in
// https://github.com/crypto-org-chain/cronos/wiki/V1.4-Benchmark
//
// Extends benchmark-3val.jsonnet with 2 more validators; the wiki does not
// list separate mempool/gas options for 5 validators, so all non-validator
// fields are kept identical to the 3-validator config.
{
  dotenv: '../../../../scripts/.env',
  'cronos_777-1': {
    cmd: 'cronosd',
    'start-flags': '--trace --async-check-tx',
    config: {
      db_backend: 'rocksdb',
      event_bus_buffer_capacity: 128,
      rpc: {
        'max_open_connections': 10000,
      },
      mempool: {
        size: 100000,
        recheck: false,
        type: 'app',
        broadcast: true,
        reap_max_gas: 363000000,
        reap_interval: '500ms',
      },
      consensus: {
        timeout_commit: '20ms',
        // A full 363000000-gas block takes longer to propagate and validate
        // across every validator than the 3s default allows, so round 0's
        // proposal always arrived late: every load height prevoted nil, burned
        // ~8.5s in propose/prevote/precommit timeouts, and re-ran
        // ProcessProposal to commit the same block at round 1. This is an
        // upper bound, not a fixed delay - a prompt proposal still commits
        // immediately, so empty blocks stay fast.
        timeout_propose: '15s',
      },
      tx_index: {
        indexer: 'null',
      },
      p2p: {
        libp2p: {
          enabled: true,
          // 256 matches go-libp2p's hardcoded QUIC concurrent-stream ceiling,
          // which is what actually binds - the resource manager's per-peer
          // stream limit is never reached, so a higher value here buys nothing.
          // The stall this config originally tried to work around came from
          // lp2p.Peer.Send opening a stream per envelope; that is fixed in the
          // transport by multiplexing envelopes over one stream per channel.
          limits: {
            mode: 'custom',
            max_peers: 16,
            max_peer_streams: 256,
          },
        },
      },
    },
    'app-config': {
      chain_id: 'cronos_777-1',
      'app-db-backend': 'rocksdb',
      'minimum-gas-prices': '0basetcro',
      'index-events': ['ethereum_tx.ethereumTxHash'],
      'iavl-lazy-loading': true,
      'async-check-tx': true,
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
        'max-txs': 100000,
        recheck: false,
        type: 'app',
        broadcast: true,
        reap_max_gas: 363000000,
        reap_interval: '500ms',
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
        'mempool-txs-per-block': 17285,
        'tx-cache-size': 100000,
      },
    },
    // 5 uniform validators, all rocksdb-backed
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
