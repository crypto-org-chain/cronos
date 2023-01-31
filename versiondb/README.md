# VersionDB

VersionDB stores multiple versions of on-chain state key-value pairs directly, without using a merklized tree structure like IAVL tree, both db size and query performance are much better than IAVL tree. The major lacking feature compared to IAVL tree is root hash and merkle proof generation, we still need IAVL tree for those tasks.

Currently grpc query service don't support proof generation, so versiondb alone is enough to support grpc query service, there's already a `--grpc-only` flag for one to start a standalone grpc query service.

There could be different implementations for the idea of versiondb, the current implementation we choose is based on rocksdb v7's experimental user-defined timestamp[^1], it'll be stored in a separate rocksdb instance, don't support other db backend yet, but the other databases in the node still support multiple backends as before.

After versiondb is enabled, there's no point to keep the full archived IAVL tree anymore, it's recommended to prune the IAVL tree to keep only recent versions, for example versions within the unbonding period or even less.

## Configuration

To enable versiondb, add `versiondb` to the list of `store.streamers` in `app.toml` like this:

```toml
[store]
streamers = ["versiondb"]
```

On startup, the node will create a `StreamingService` to subscribe to latest state changes in realtime and save them to versiondb, the db instance is placed at `$NODE_HOME/data/versiondb` directory, there's no way to customize the db path currently. It'll also switch grpc query service's backing store to versiondb from iavl tree, you should migrate the legacy states in advance to make the transition smooth, otherwise, the grpc queries can't see the legacy versions.

If the versiondb is not empty and it's latest version doesn't match the multistore's last committed version, the startup will fail with error message `"versiondb lastest version %d doesn't match iavl latest version %d"`, that's to avoid creating gaps in versiondb accidentally. When this error happens, you just need to update versiondb to the latest version in iavl tree manually (see [](#catch-up-with-iavl-tree)).

## Migration

Since our chain has pretty big now, a lot of efforts have been put to make sure the transition process can finish in practical time. The migration process will try to parallelize the tasks as much as possible, and use significant ram, but there's flags for user to control the concurrency level and ram usage to make it runnable on different machine specs.

The legacy state migration process is done in two main steps:

- Extract state change sets from existing archive IAVL tree.
- Feed the change set files to versiondb.

### Extract Change Sets

```bash
$ export STORES="distribution acc authz bank capability cronos evidence evm feegrant feeibc feemarket gov ibc mint params slashing staking transfer upgrade"
$ cronosd changeset dump --home /chain/.cronosd/ --output data $STORES
```

`dump` command will extract the change sets from the IAVL tree, and store each store in separate directories. The change set files are segmented into different chunks and compressed with zlib level 6 by default, the chunk size defaults to 1m blocks, the result `data` directly will look like this:

```
data/acc/block-0.zz
data/acc/block-1000000.zz
data/acc/block-2000000.zz
...
data/authz/block-0.zz
data/authz/block-1000000.zz
data/authz/block-2000000.zz
...
```

Extraction is the slowest step, the test run on testnet archive node takes around 11 hours on a 8core ssd machine, but fortunately, the change set files can be verified pretty fast, so they can be share on CDN in a trustless manner, normal users should just download them from CDN and verify the correctness locally, should be much faster than extract by yourself.

For rocksdb backend, `dump` command opens the db in readonly mode, it can run on live node's db, but goleveldb backend don't support this.

#### Verify Change Sets

```bash
$ cronosd changeset verify data/acc/*.zz
7130689 8DF52D6F7A7690916894AF67B07D64B678FB686626B2B3109813BBE172E74F08
```

`verify` command will replay all the change sets and rebuild the final IAVL tree and output the root hash of the final version, user can check the root hash against the IAVL tree.

> TODO: will provide command to generate app-hash directly, combining the root hashes of each store, so it'll be easier for end user to verify.

`verify` command takes several minutes and several gigabytes of ram to run, if ram usage is a problem, it can also run incrementally, you can export the snapshot for a middle version, then verify the remaining versions start from that snapshot:

```bash
$ cronosd changeset verify --save-snapshot /tmp/snapshot data/acc/block-0.zz data/acc/block-1000000.zz data/acc/block-2000000.zz
$ cronosd changeset verify --load-snapshot /tmp/snapshot data/acc/block-3000000.zz data/acc/block-4000000.zz data/acc/block-5000000.zz
```

The format of change set files are documented [here](memiavl/README.md#change-set-file).

### Convert To VersionDB

#### SST File Writing

To maximize the speed of initial data ingestion into rocksdb, we take advantage of the sst file writer in rocksdb, with that we can write out sst files directly without causing contention on a shared database, the sst files for each store can be written out in parallel. We also developed an external sorting algorithm to sort the data before writing the sst files, so the sst files don't have overlaps and can be ingested into the bottom-most level in db.

```bash
$ # convert a single store
$ cronosd changeset convert-to-sst --store distribution ./sst/distribution.sst data/distribution/*.zz

$ # convert all stores sequentially
$ for store in $STORES
> do
> cronosd changeset convert-to-sst --store $store ./sst/$store.sst data/$store/*.zz
> done
```

You can also wrap it in a simple script to run multiple stores in parallel. Here's an example Python script:

```python
import os
import sys
from datetime import datetime
from pathlib import Path
import subprocess
from concurrent.futures import ThreadPoolExecutor, as_completed

CRONOSD = './result/bin/cronosd'
DEFAULT_STORES = "distribution acc authz bank capability cronos evidence evm feegrant feeibc feemarket gov ibc mint params slashing staking transfer upgrade".split()

SOURCE = Path("./data")
DEST = Path("./sst")

# translate to around 2-3G peak ram usage for each task,
# tune it according to available ram and concurrency level
SORT_CHUNK_SIZE = 256*1024*1024


def run_store(executor, store, srcd, dstd):
    sst = dstd / f'{store}.sst'
    inputs = [src for src in (srcd / store).glob('block-*')]
    return executor.submit(subprocess.run, [CRONOSD, 'changeset', 'convert-to-sst', '--store', store, '--sorter-chunk-size', str(SORT_CHUNK_SIZE), sst] + inputs, check=True)

def main(stores, concurrency):
    DEST.mkdir(exist_ok=True, parents=True)
    with ThreadPoolExecutor(max_workers=concurrency) as executor:
        futs = []
        for store in stores:
            futs.append(run_store(executor, store, SOURCE, DEST))
        for fut in as_completed(futs):
            fut.result()

if __name__ == '__main__':
    try:
        stores = sys.argv[1].split()
    except IndexError:
        stores = DEFAULT_STORES
    main(stores, os.cpu_count())
```

User can control the peak ram usage by controlling the parallel level and `--sorter-chunk-size`.

The provided python script can finish in around 20minutes for testnet archive node.

#### SST File Ingestion

Finally, we can ingest the generated sst files into final versiondb:

```bash
$ cronosd changeset ingest-sst /chain/.cronosd/data/versiondb/ sst/*.sst --maximum-version 7130689 --move-files
```

This command takes around 1 second to finish, `--move-files` will move the sst files instead of copy them, `--maximum-version` specifies the maximum version in the change sets, it'll override the existing latest version if it's bigger,
the sst files will be put at bottom-most level possible, because the generation step make sure there's no key overlap between them.

#### Catch Up With IAVL Tree

If an non-empty versiondb lags behind from the current IAVL tree, the node will refuse to startup, in this case user need to manually sync them, the steps are quite similar to the migration process since genesis:

- Stop the node so it don't process new blocks.

- Dump change sets for the block range between the latest version in versiondb and iavl tree, just specify the `--start-version` parameter to versiondb's latest version plus one:

  ```bash
  $ cronosd changeset dump --home /chain/.cronosd/ --output /tmp/data --start-version 7241675 $STORES
  ```

- Feed the change set to versiondb, here we can skip the sst file writer generation, that is for parallel processing of big amount of change sets, since we are dealing with much smaller amount here, this one should be fast enough:

  ```bash
  $ for store in $STORES
  > do
  > ./result/bin/cronosd changeset to-versiondb /chain/.cronosd/data/versiondb data2/$store/*.zz --store $store
  > done
  ```

- Finally, use `ingest-sst` command to update the lastest version, just don't pass any sst files:

  ```bash
  $ cronosd changeset ingest-sst /chain/.cronosd/data/versiondb/ --maximum-version 7300922
  ```

Of course, you can always follow the sst file generation and ingestion process if the data amount if big.



[^1]: https://github.com/facebook/rocksdb/wiki/User-defined-Timestamp-%28Experimental%29
