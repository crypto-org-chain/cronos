# **Executive Summary**

# 

# **Performance Benchmarking and Network Optimization (Cronos EVM v1.8)**  **Status:** Draft · **Revised:** Jul 7, 2026 · **Owner:** [Thomas Nguy](mailto:thomas.nguy@cronos.com)

##  **1\. Summary**

This proposal outlines a comprehensive 3-week benchmarking initiative for the upcoming major upgrade of the Cronos network, specifically focusing on the Cronos EVM v1.8 binary. Initial local testing has demonstrated outstanding performance gains, showing up to 103% reduction in block execution time. To ensure mainnet stability, validate scalability metrics under production-grade conditions, and resolve historical consensus timeout complaints, we request a budget of $6,000 USD to provision an AWS-based Devnet environment that mirrors our live production specifications.

## **2\. Strategic Context & Key Enhancements**

The v1.8 binary introduces architectural improvements to the Cronos base layer. These changes target node communication, block construction efficiency, execution throughput, and overall system robustness.

 Key enhancements include:

> * **Reduced P2P Latency:** The integration of the libp2p networking stack drastically reduces peer-to-peer latency, providing structural improvements to network topology, node discovery, and overall protocol resilience.  
> * **Application-Side Mempool:** Introduction of a custom pre-verification layer on the application side. This bypasses the heavy verification overhead traditionally processed during block construction and execution, facilitating faster block turnaround.  
> * **Native Block-STM:** A highly optimized, mature implementation of Parallel EVM execution released last year, now directly embedded within the Cosmos SDK layer for high-concurrency smart contract execution.  
> * **EVM API Equivalence:** Extension of coverage to over 100 individual Ethereum API methods, successfully driving Cronos’s EVM API equivalence to above 90%, ensuring seamless compatibility for Ethereum-native developer tooling.  
> * **Bug Fixes & Stability:** Over 40 critical bug fixes have been upstreamed and integrated to prevent regression and improve long-term state stability.

## **3\. Problem Statement & Justification**

While early evaluation on single-node local laptops confirms substantial optimizations, local testing cannot accurately replicate the intricate dynamics of a distributed, multi-validator global network. Production networks encounter network jitter, real-world latency, state bloat, and competitive mempool behavior. Without a dedicated Devnet that mirrors our production infrastructure, it is impossible to stress-test the network safely, discover edge-case bottlenecks, or confidently guarantee the stability of the upgrade prior to public mainnet deployment.

## **4\. Benchmarking Objectives & Goals**

The primary objectives of this 3-week benchmarking infrastructure are as follows:

> 1. **Load and Stability Evaluation:** Monitor and analyze the structural integrity and stability of the new binary under continuous, heavy synthetic transaction traffic to catch potential memory leaks, deadlocks, or state divergence.  
> 2. **Maximum TPS Determination:** Establish the true peak Transactions Per Second (TPS) of the network under realistic mainnet constraints. The final data will compare the v1.8 binary directly against the current v1.7 baseline, stating the final optimization gains as an explicit percentage improvement.  
> 3. **Network Parameter Tuning:** Systematically adjust and fine-tune consensus parameters to isolate the most optimal configuration for throughput and latency under constrained real-world networking topologies.  
> 4. **Validate Block Time Improvements:** Validate improvements during the consensus phase. In the current production environment, minor complaints have surfaced regarding block production delays taking up to 4 seconds due to consensus timeouts. The v1.8 binary is strategically positioned to resolve this, and the Devnet will objectively measure the reduction in timeout occurrences. If this is validated, the latency for Cronos app traders will be improved.

## **5\. Methodology & Testing Tools**

To achieve reproducible and professional load generation, the engineering team will leverage two primary internal toolsets:

> * **Cronos Transaction Bot:** This utility functions as an automated service to simulate organic network traffic. It executes scheduled bursts of transactions across multiple test accounts, automatically manages gas funds to maintain consistent load, and features a gas-mirroring system that reads live mainnet blocks to accurately simulate realistic smart contract execution loads within the benchmarking environment.

>   [https://github.com/crypto-org-chain/cronos-tx-bot](https://github.com/crypto-org-chain/cronos-tx-bot)

> * **Testground Framework:** A benchmarking and containerized orchestration framework built specifically for network load testing. It enables firing precise, configurable transaction types (such as asset transfers or ERC-20 interactions) while gathering deep performance analytics, block statistics, and state metrics.

>   [https://github.com/crypto-org-chain/cronos/tree/main/testground](https://github.com/crypto-org-chain/cronos/tree/main/testground)

> 

## **6\. Implementation Plan & Timeline**

The benchmarking will follow an incremental, phased approach over 3 weeks immediately following budget approval and environment provisioning:

### **Phase 1: Baseline Comparison (v1.7 vs. v1.8)**

First, the Devnet will be configured using existing production/mainnet parameters to establish a strict baseline utilizing the current v1.7 binary. The baseline parameters are configured as follows:

```shell
timeout_propose = "3s"timeout_propose_delta = "500ms"timeout_prevote = "1s"timeout_prevote_delta = "500ms"timeout_precommit = "1s"timeout_precommit_delta = "500ms"create_empty_blocks_interval = "5s"peer_gossip_sleep_duration = "100ms"peer_query_maj23_sleep_duration = "2s"timeout_commit = "200ms"
```

The team will then execute the following procedural checklist:

> * Generate synthetic traffic via the Cronos Transaction Bot under v1.7 parameters.  
> * Utilize the Testground framework to record baseline block execution statistics.  
> * Upgrade the Devnet nodes to the new v1.8 binary.  
> * Rerun identical traffic loads to capture initial head-to-head performance gains.  
> * Explicitly enable libp2p and application-side mempool optimizations on v1.8 to measure isolated architectural gains.  
> * Storage: gp3 3000 IOPS 300 MiB/s.  
> * Same voting power for validators.

**State Contention Testing for Block-STM:**   
Conduct focused stress tests introducing high-contention traffic workloads. Since Parallel EVM efficiency depends on state independence, this sub-phase will explicitly push concurrent transactions attempting to modify identical smart contract states (mimicking hot DeFi AMM pools or viral NFT mints) to ensure the parallel scheduler minimizes re-execution overhead.

### **Phase 2: Network Parameter Tuning & Consensus Diagnostics**

With v1.8 active, the team will systematically alter core network consensus parameters while continuously running the transaction bot to find the maximum optimal performance boundary. This phase gives us a valuable opportunity to safely tune these parameters without affecting the live mainnet, ultimately helping us resolve the current consensus round bottleneck.

Target Parameters for Tuning: 

```shell
timeout_commit
timeout_prevote
timeout_prevote_delta
timeout_precommit
timeout_precommit_delta
```

> * **Advanced Consensus Analysis:** During this phase, INFO and DEBUG logs will be extensively collected. The incremental testing framework will focus specifically on breaking down consensus round times, tracking down micro-bottlenecks, and mapping out the precise causes of block timeouts.

>   
   ●    **Telemetry & Resource Utilization Metrics** : Expand diagnostic observation beyond basic throughput counts to analyze node system-level vitals via a Prometheus/Grafana stack.   
Testing will record data on:  
1\) Memory (RAM) Footprint: To rule out any long-duration heap memory leaks across nodes coming from the libp2p migration.  
2\) Disk I/O and IOPS Bottlenecks: To confirm that rapid concurrent executions processed by Block-STM do not saturate underlying node database hardware.  
3\) Network Bandwidth Efficiency: Benchmarking total packet data flows to verify network optimization with node exporter. [Reference](https://github.com/prometheus/node_exporter%20)

## **7\. Deliverables**

A report of Cronos v1.8 performance documenting

1. The peak TPS gains of the v1.8 binary relative to the v1.7 baseline.  
2. Technical analysis of consensus block time stability data and system resource utilization (CPU, RAM, I/O, Network) under different network parameters.  
3. Recommendation to mainnet consensus network parameters (If applicable).

## 

## **8\. Budget & Financial Estimation**

To replicate a highly accurate production environment consisting of high-compute validators and full nodes, we propose provisioning the Devnet on AWS.

| Resource Component | Duration | Maximum Estimated Cost (USD)   |
| :---- | :---- | ----- |
| AWS Production-Spec Devnet (Compute, Storage, & Networking) | 3 Weeks | $5.563.00 |
| **Total Budget Requested** | **3 Weeks** | **$5,563.00** |

**Cost Optimization Strategy:** We are enforcing a hard spending cap of up to $6,000 USD for this project. This figure represents the absolute upper ceiling based on continuous, 24/7 server operations under mainnet specifications. In practice, the engineering team will drastically reduce the actual spend by implementing automated scripts to turn off compute resources during non-testing hours and weekends.

# **Cronos Tuning Guide**

# **Cronos EVM Tuning Guide** 

## **1\. Mathematical Framework of EVM Throughput**

**EVM throughput** is treated not as a static variable, but as an emergent system property. We define the maximum sustainable transaction-per-second capability (**TPSmax**):

**TPSmax \= Ntx / tblock**

Where:

* **Ntx** is the transaction volume committed per block.  
* **tblock** is the total processing time of a block at steady state.

### 

### **1.1. Block Capacity Limits (Ntx)**

The maximum **transaction allocation** per block is bounded by three competing ceilings:

**Ntx \= min( ⌊ Gblock / gtx ⌋, Bbytes, Mtx )**

* **Gblock** is the block gas limit (consensus.params.block.max\_gas).  
* **gtx** is the execution gas consumed per transaction (e.g., **21,000** gas for simple-transfer; **51,630** gas for erc20-transfer).  
* **Bbytes** is the payload capacity ceiling, defined by:

**Bbytes \= ⌊ block.max\_bytes / avg\_tx\_size ⌋**

* **Mtx** is the application memory-mempool processing cap (mempool-txs-per-block).

### **1.2. Temporal Block Budget (tblock)**

The elapsed time between block proposals must be minimized. It is decomposed into **synchronous execution**, **serialization**, and **consensus delays**:

**tblock \= tFinalizeBlock \+ tCommit \+ tConsensus \+ tPropagation**

Where:

* **tFinalizeBlock** is the total time required for the EVM execution engine to process all transaction state transitions.  
* **tCommit** is the time required to write state transitions to the tree structure and commit them to persistence.  
* **tConsensus** is the sum of voting-round latencies (Propose, Prevote, and Precommit phases).  
* **tPropagation** is the network transit and block-part reconstruction time.

## **2\. Measurement Methodology**

To ensure benchmarking data is reliable and repeatable, all experimental runs must adhere to these measurement protocols:

### **2.1. Steady-State Evaluation**

Performance measurements must exclude the system's warm-up and cooldown phases. The active analysis window is defined as:

**Sustained Window \= Ttotal \- Twarmup \- Tdrain**

* **Headline Metric:** Report the **median TPS** over this sustained window.  
* **Rejection of Peak Anomalies:** Do not use peak\_tps as a performance rating, as it represents outlier blocks that do not reflect sustainable capacity.

### **2.2. Outlier Rejection**

Isolate **consensus delays** and **garbage collection pauses** from core engine measurements. Exclude blocks whose intervals exceed the 25th-percentile block time by more than a factor of five:

**Filter Condition: tblock \> 5 × P25( tblock )**

### **2.3. Hybrid Telemetry Source**

Avoid using timestamp data from the **load generator**, as network propagation delay can skew results. Instead, query block data using the **hybrid telemetry model**:

* **Timestamps:** Query sub-second block header timestamps from the Cosmos SDK RPC (get\_block\_info\_hybrid).  
* **Transaction Counts & Gas:** Query committed transaction numbers and actual gas consumed from the Ethereum JSON-RPC (eth\_getBlockByNumber).

### **2.4. Network Saturation Verification**

A benchmark run is only valid as a "**maximum limit test**" if the network is fully **saturated**. Confirm that the following conditions are met before recording results:

1. **Gas Utilization Rate:** **Gasutilized / Gblock ≥ 90%** (the block space is fully occupied).  
2. **Mempool Saturation:** **Mempoolpending \> 0** throughout the entire run (there is a continuous queue of pending transactions).  
3. **Transaction Success Rate:** **Failed Transactions \< 1%** (high failure rates point to generator pacing issues or account sequence errors, not execution limits).

## 

## **3\. Phased Tuning & Optimization Plan**

### 

### **Phase 1: Core Performance Engine (Execution & Storage)**

In this phase, we establish and optimize our baseline performance engine. Tuning focuses on optimizing thread allocation and storage cache settings to minimize both **tFinalizeBlock** and **tCommit**.

| Configuration Key | File Location | Baseline Value | Sweep Range | Expected Outcome & Tradeoff |
| :---- | :---- | :---- | :---- | :---- |
| block-executor | app.toml \[evm\] | **"block-stm"** | *Fixed* | Baseline starting point for parallel execution. |
| memiavl.enable | app.toml \[memiavl\] | **true** | *Fixed* | Baseline starting point for in-memory state commitment. |
| rocksdb.node\_type | app.toml \[rocksdb\] | **"validator"** | *Fixed* | Baseline starting point for low-latency point lookups. |
| block-stm-workers | app.toml \[evm\] | 0 (auto-scale) | 0 \-\> 4 \-\> 8 \-\> 16 | Optimizes parallel threads. Exceeding physical core limits can cause context switching overhead. |
| block-stm-pre-estimate | app.toml \[evm\] | false | false \-\> true | Pre-estimates read/write sets to minimize aborts when **Rreexec \> 1.0**. |
| memiavl.async-commit-buffer | app.toml \[memiavl\] | 0 | 0 \-\> 16 \-\> 32 \-\> 64 | Optimizes the commit queue buffer to process state writes asynchronously. |
| memiavl.cache-size | app.toml \[memiavl\] | 1000 | 1000 \-\> 100k \-\> 1M | Speeds up read operations. Requires additional RAM allocation. |
| memiavl.zero-copy | app.toml \[memiavl\] | false | false \-\> true | Bypasses memory allocation copying for direct state access. |
| db\_backend | config.toml | goleveldb | goleveldb \-\> rocksdb | Configures RocksDB as the storage engine for the transaction store. |

### 

### 

### **Phase 2: Block Capacity Tuning (Gas & Mempool Limits)**

This phase raises block-level capacity ceilings to maximize the transaction count (**Ntx**) per block.

| Configuration Key | File Location | Baseline Value | Sweep Range | Expected Outcome & Tradeoff |
| :---- | :---- | :---- | :---- | :---- |
| max\_gas | genesis.json | \-1 | 105M \-\> 210M \-\> 400M \-\> 800M \-\> \-1 | Increases the transaction execution budget per block. |
| max\_bytes | genesis.json | 21 MB | 21MB \-\> 50MB \-\> 100MB | Prevents block size limits from throttling high-density runs. |
| mempool-txs-per-block | app.toml \[cronos\] | 2900 | 2900 \-\> 10000 \-\> 20000 \-\> 0 (Unlimited) | Expands the application-level transaction limit per block. |
| reap\_max\_gas | config.toml \[mempool\] | 0 | Keep 0 (or align with max\_gas) | Ensures transaction harvesting limits match the block gas capacity. |

### 

### 

### **Phase 3: Protocol Pipeline Cadence (Consensus & P2P Mempool)**

This phase minimizes consensus delay (**tConsensus**) and network propagation time (**tPropagation**) to speed up block times and maximize block-propagation rates.

| Configuration Key | File Location | Baseline Value | Sweep Range | Expected Outcome & Tradeoff |
| :---- | :---- | :---- | :---- | :---- |
| timeout\_commit | config.toml \[consensus\] | 1s | 1s \-\> 200ms \-\> 50ms \-\> 20ms \-\> 0 | Minimizes empty block wait states. |
| skip\_timeout\_commit | config.toml \[consensus\] | false | false \-\> true | Triggers block proposals immediately once \+2/3 of precommits are received. |
| timeout\_propose | config.toml \[consensus\] | 3s | 3s \-\> 1s \-\> 500ms | Reduces wait times for block proposals. |
| timeout\_prevote / timeout\_precommit | config.toml \[consensus\] | 1s / 1s | 1s/1s \-\> 500ms/500ms \-\> 300ms | Speeds up the consensus voting phase. |
| mempool.type | config.toml \[mempool\] | fixed | flood \-\> app | Swaps default gossip routing for direct app mempool processing. |
| mempool.max-txs | app.toml \[mempool\] | 5000 | 5000 \-\> 50k \-\> 200k | Same above, but for app mempool only. |
| mempool.tx-cache-sizetx-cache-size | app.toml    \[cronos\] | auto-derived | Same above | Cache tx for better execution speed. With cached tx, avoiding the heavy computation during the executing in block process. |
| mempool.recheck | config.toml \[mempool\] | true | true \-\> false | Disables post-commit transaction rechecks to reduce CPU overhead. |
| send\_rate / recv\_rate(CometP2P Only) | config.toml \[p2p\] | 5 MB/s | 5 MB/s \-\> 50 MB/s \-\> 100 MB/s | Prevents network interface throttling during block propagation. |
| Max\_packet\_msg\_payload\_size(CometP2P Only) | config.toml \[p2p\] | 1024 | 1024 \-\> 65536 | Minimizes network packet fragmentation. |
| Libp2p configuration | Refers to v1.8 upgrade guide | Refers to v1.8 upgrade guide | Refers to v1.8 upgrade guide | Refers to v1.8 upgrade guide |

## 

## 

For libp2p configuration, refers to  [Cronos v1.8 Upgrade Guide](https://docs.google.com/document/d/1ilfzrS3KD1jBeBme9if73Xwq4vfD0yjJMRz3eycoPkA/edit?tab=t.0#heading=h.qttax13ch3yx)

# **Schedule**

# **Cronos EVM Benchmarking Execution Schedule**

## **Phase 1: TPS Comparison (v1.7 vs. v1.8 Baseline)**

The performance improvements in Cronos v1.8 focus heavily on the **P2P propagation layer** and the **block finalize pipeline**.

If we benchmarked these versions using highly optimized, aggressive network variables immediately:

1. We would mask the real-world behavior of the upgrade under standard network constraints.  
2. We would be unable to measure the actual performance delta that node operators will experience on the live network.

Using Mainnet values first ensures we isolate the exact efficiency gains of the new codebase under production-realistic traffic and latencies before pushing the configuration parameters to their absolute limit.

| Step | Task Name | Actionable DevOps Command / Objective | Configuration Target | Who |
| :---- | :---- | :---- | :---- | :---- |
| **1** | **v1.7 Env Provisioning** | Spin up validator cluster (15 nodes) using the **Cronos v1.7** binary. | Apply standard Mainnet configuration variables. Enable block-stm with pre-estimate(block-stm-pre-estimate \= true) Block gas limit to 60M |  |
| **2** | **v1.7 Baseline Run** | Warm up caches (ignore first 5 blocks).  Send only \`CRO transfer\` transactions to maximize block gas | Measure TPS  |  |
| **3** | **v1.7 Stress Sweep** | Incrementally increase the block gas limit  | Stop when block time degrades.  Measure TPS  |  |
| **4** | **v1.8 Env Provisioning** | Re-flash cluster with the **Cronos v1.8** binary. | Apply standard Mainnet configuration variables. Enable block-stm with pre-estimate Block gas limit to 60M |  |
| **5** | **v1.8 Stress Sweep** | Repeat Step 2 and Step 3 using the v1.8 binary under the exact same load profile. | Stop when block time degrades.  Measure TPS |  |

### 

## **Phase 2: Fine-Tuning v1.8**

**Objective:** Apply the aggressive Cronos Tuning Guide configurations to the v1.8 binary to find the network's absolute performance ceiling.

See [Cronos EVM v1.8 Performance Benchmarking and Network Optimization](https://docs.google.com/document/d/1DT2yzoqibC97a6bvzlN3xZK-q8BX5JkZM-NA5QlrTCM/edit?tab=t.y76u4smps2i)

| Step | Tuning Layer | DevOps Config Changes to Apply | Target Operational Outcome |
| :---- | :---- | :---- | :---- |
| **1** | **Establish Baseline** | Initialize v1.8 cluster with the highest-throughput baseline configuration verified in Phase 1\. | Baseline checkpoint. |
| **2** | **Storage Layer** | Apply to app.toml \[memiavl\], \[rocksdb\], and config.toml:memiavl.enable \= true memiavl.cache-size \= 1000000 memiavl.zero-copy \= true rocksdb.node\_type \= "validator" db\_backend \= "rocksdb" | Drastically reduce state commit time |
| **3** | **Consensus Layer** | Apply to config.toml \[consensus\]: timeout\_propose        \= "500ms"  timeout\_propose\_delta  \= "500ms"timeout\_prevote        \= "250ms"  timeout\_prevote\_delta  \= "500ms"timeout\_precommit      \= "250ms"timeout\_precommit\_delta \= "500ms"timeout\_commit         \= "200ms" create\_empty\_blocks          \= truecreate\_empty\_blocks\_interval \= "0s" peer\_gossip\_sleep\_duration \= "50ms" peer\_query\_maj23\_sleep\_duration \= "2s"skip\_timeout\_commit \= true  Reduce vote round-trip timeouts (timeout\_propose, timeout\_prevote, timeout\_precommit). | Minimize idle wait overhead and consensus delay  |
| **4** | **Mempool & P2P** | Apply to config.toml \[mempool\], \[p2p\]:mempool.type \= "app"mempool.recheck \= falsemempool-txs-per-block \= 0tx-cache-size \= 0  | Prevent propagation bottlenecks  and reduce CPU overhead. |
| **5** | **Final Stress Runs** | Execute 3 continuous runs under full saturation using the fully optimized master profile. | Consolidate telemetry to verify target metrics stability. |

