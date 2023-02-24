# Alternative IAVL Implementation

## Changelog

* 11 Jan 2023: Initial version
* 13 Jan 2023: Change changeset encoding from protobuf to plain one
* 17 Jan 2023:
  * Add delete field to change set to support empty value
  * Add section about compression on snapshot format
* 27 Jan 2023:
  * Update metadata file format
  * Encode key length with 4 bytes instead of 2.
* 24 Feb 2023:
  * Reduce node size (without hash) from 32bytes to 16bytes.
  * Append offset table to the end of keys/values file, try elias-fano coding for the values one.


## The Journey

It started for an use case of verifying the state change sets, we need to replay the change sets to rebuild IAVL tree and check the final IAVL root hash, compare the root hash with the on-chain hash to verify the integrity of the change sets.

The first implementation keeps the whole IAVL tree in memory, mutate nodes in-place, and don't update hashes for the intermediate versions, and one insight from the test run is it runs surprisingly fast. For the distribution store in our testnet, it can process from genesis to block `6698242` in 2 minutes, which is around `55818` blocks per second.

To support incremental replay, we further designed an IAVL snapshot format that's stored on disk, while supporting random access with mmap, which solves the memory usage issue, and reduce the time of replaying.

## New Design

So the new idea is we can put the snapshot and change sets together, the change sets is the write-ahead-log for the IAVL tree.

It also integrates well with versiondb, because versiondb can also be derived from change sets to provide query service. IAVL tree is only used for consensus state machine and merkle proof generations.

### Advantages

- Better write amplification, we only need to write the change sets in real time which is much more compact than IAVL nodes, IAVL snapshot can be created in much lower frequency.
- Better read amplification, the IAVL snapshot is a plain file, the nodes are referenced with offset, the read amplification is simply 1.
- Better space amplification, the archived change sets are much more compact than current IAVL tree, in our test case, the ratio could be as large as 1:100. We don't need to keep too old IAVL snapshots, because versiondb will handle the historical key-value queries, IAVL tree only takes care of merkle proof generations for blocks within an unbonding period. In very rare cases that do need IAVL tree of very old version, you can always replay the change sets from the genesis.

## File Formats

> NOTICE: the integers are always encoded with little endianness.

### Change Set File

```
version: int64
size: int64         // size of whole payload
payload:
  delete: int8
  keyLen: varint-uint64
  key
  [                 // if delete is false
    valueLen: varint-uint64
    value
  ]

repeat with next version
```

- Change set files can be splited with certain block ranges for incremental backup and restoration.

- Historical files can be compressed with zlib, because it doesn't need to support random access.

### IAVL Snapshot

IAVL snapshot is composed by four files:

- `metadata`, 16bytes:

  ```
  magic: uint32
  format: uint32
  version: uint32
  root node index: uint32
  ```

- `nodes`, array of fixed size(16+32bytes) nodes, the node format is like this:

  ```
  height   : uint32         // padded to 4bytes
  version  : uint32
  size     : uint32
  key_node : uint32
  hash     : [32]byte
  ```
  The node has fixed length, can be indexed directly. The nodes reference each other with the node index, nodes are written in post-order, so the root node is always placed at the end.

  For branch node, the `key_node` field reference the smallest leaf node in the right branch, for leaf node, it's the leaf index, which can be used to find key and value in `keys` and `values` file.

  The branch node don't need to reference left/right children explicitly, they can be derived from existing information and properties of post-order traversal:

  ```
  right child index = self index - 1
  left child index = key_node - 1
  ```

  The version/size/node indexes are encoded with `uint32`, should be enough in foreseeable future, but could be changed to `uint64` in the future.

  The implementation will read the mmap-ed content in a zero-copy way, won't use extra node cache, it will only rely on the OS page cache.

- `keys`, sequence of leaf node keys, ordered and no duplication, the offsets are appended to the end of the file, user can look up the key offset by leaf node index.

  ```
  payload
  *repeat*
  key offset: uint32
  *repeat*
  offset: uint64    // begin offset of the offsets table
  ```

- `values`, sequence of leaf node values, the offsets are encoded with elias-fano coding and appended to the end of the file, user can look up the key offset by leaf node index.

  ```
  payload
  *repeat*
  offsets table encoded with elias-fano coding
  offset: uint64    // begin offset of the offsets table
  ```

#### Compression

The items in snapshot reference with each other by file offsets, we can apply some block compression techniques to compress keys and values files while maintain random accessbility by uncompressed file offset, for example zstd's experimental seekable format[^1].

### VersionDB

[VersionDB](../README.md) is to support query and iterating historical versions of key-values pairs, currently implemented with rocksdb's experimental user-defined timestamp feature, support query and iterate key-value pairs by version, it's an alternative way to support grpc query service, and much more compact than IAVL trees, similar in size with the compressed change set files.

After versiondb is fully integrated, IAVL tree don't need to serve queries at all, it don't need to store the values at all, just store the value hashes would be enough.

[^1]: https://github.com/facebook/zstd/blob/dev/contrib/seekable_format/zstd_seekable_compression_format.md
