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
  version: uint64
  root node index: uint32
  ```

- `nodes`, array of fixed size(64bytes) nodes, the node format is like this:

  ```
  height  : uint8          // padded to 4bytes
  version : uint32
  size    : uint64
  key     : uint64         // offset in keys file
  left    : uint32         // inner node only
  right   : uint32         // inner node only
  value   : uint64 offset  // offset in values file, leaf node only
  hash    : [32]byte
  ```
  The node has fixed length, can be indexed directly. The nodes reference each other with the index, nodes are written in post-order, so the root node is always placed at the end.

  Some integers are using `uint32`, should be enough in forseeable future, but could be changed to `uint64` to be safer.

  The implementation will read the mmap-ed content in a zero-copy way, won't use extra node cache, it will only rely on the OS page cache.

- `keys`, sequence of length prefixed leaf node keys, ordered and no duplication.

  ```
  size: uint32
  payload
  *repeat*
  ```

  Key size is encoded in `uint32`, so the maximum key length supported is `1<<32-1`, around 4G.

- `values`, sequence of length prefixed leaf node values.

  ```
  size: uint32
  payload
  *repeat*
  ```

  Value size is encoded in `uint32`, so maximum value length supported is `1<<32-1`, around 4G.

#### Compression

The items in snapshot reference with each other by file offsets, we can apply some block compression techniques to compress keys and values files while maintain random accessbility by uncompressed file offset, for example zstd's experimental seekable format[^1].

### VersionDB

[VersionDB](../README.md) is to support query and iterating historical versions of key-values pairs, currently implemented with rocksdb's experimental user-defined timestamp feature, support query and iterate key-value pairs by version, it's an alternative way to support grpc query service, and much more compact than IAVL trees, similar in size with the compressed change set files.

[^1]: https://github.com/facebook/zstd/blob/dev/contrib/seekable_format/zstd_seekable_compression_format.md