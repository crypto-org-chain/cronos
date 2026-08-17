// 5-validator devnet config matching the "5 Validators" setup described in
// https://github.com/crypto-org-chain/cronos/wiki/V1.4-Benchmark
//
// Derived from benchmark-3val.jsonnet with 2 more validators. The wiki lists no
// separate 5-validator options, so the tuning here was found by measurement and
// has since diverged from the 3-validator config on purpose: max block gas is
// 210000000 rather than 363000000 (a 363M block does not propagate across five
// validators inside the round timeouts), the consensus timeouts are the
// mainnet-realistic set rather than 3val's 20ms commit, mempool.max-txs is
// sized for unique-per-tx, and libp2p carries explicit resource limits.
// Do not "resync" those with 3val - each is load-bearing and commented below.
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
        reap_max_gas: 210000000,
        reap_interval: '500ms',
      },
      consensus: {
        // Phase 1 baseline: mainnet-realistic timeouts (tuning-doc Phase 1 /
        // validator-config.toml). Phase 2 Layer 2 tried the aggressive values
        // (timeout_prevote/precommit 1s->500ms, deltas 500ms->200ms, commit
        // 200ms->100ms) and it regressed median_tps -2.7% - reverted here.
        // Phase 7 then tried skip_timeout_commit=true, timeout_commit=50ms and
        // peer_gossip_sleep_duration=25ms chasing a 500ms blocktime: they halve
        // the empty-block cadence (~450ms->~250ms) but change nothing under
        // load, and roughly double the sdk:3 failure rate - also reverted.
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
        // Only key app.toml's [mempool] has (cosmos-sdk MempoolConfig); the
        // type/broadcast/reap_* knobs belong to config_patch's CometBFT
        // mempool above, and a copy here is silently ignored.
        //
        // unique-per-tx puts every one of the 300000 physical senders in
        // flight at once with no per-sender pacing to throttle them; a cap
        // below that drops the overflow via broadcast_tx_async, which never
        // waits for CheckTx and so never surfaces the ErrMempoolIsFull as an
        // error the client can see or retry.
        'max-txs': 320000,
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
        'cache-size': 1000,
        'async-commit-buffer': 16,
      },
      grpc: {
        'skip-check-header': true,
      },
      cronos: {
        // Shared budget for the gossip-reap cap (txs per 500ms tick) and the
        // recheck-batch cap (candidates per Commit). 210000000 max block gas /
        // 21000 gas per simple-transfer = 10000 txs a full block can hold.
        // Leaving this 0 falls back to cronos's 2900 mainnet default, which
        // starves both paths at ~3.4x below one block's worth.
        'mempool-txs-per-block': 10000,
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
            max_gas: '210000000',
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
