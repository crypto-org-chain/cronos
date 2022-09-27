"""
cli utilities for versiondb
"""
import binascii
from pathlib import Path

import click
import rocksdb
from cprotobuf import decode_primitive
from roaring64 import BitMap64


def rocksdb_stats(path):
    db = rocksdb.DB(str(path), rocksdb.Options())
    for field in ["rocksdb.stats", "rocksdb.sstables"]:
        print(f"############# {field}")
        print(db.get_property(field.encode()).decode())

    # space amplification
    it = db.iteritems()
    it.seek_to_first()
    count = 0
    size = 0
    for k, v in it:
        count += 1
        size += len(k) + len(v)
    # directory size
    fsize = sum(f.stat().st_size for f in path.glob("**/*") if f.is_file())
    print(
        f"space_amplification: {fsize / size:.2f}, kv pairs: {count}, "
        f"data size: {size}, file size: {fsize}"
    )


@click.group()
def cli():
    pass


@cli.command()
@click.option("--dbpath", help="path of plain db")
def latest_version(dbpath):
    db = rocksdb.DB(dbpath, rocksdb.Options())
    bz = db.get(b"s/latest")
    # gogoproto std int64, the first byte is field tag
    print(decode_primitive(bz[1:], "int64")[0])


@cli.command()
@click.option("--dbpath", help="path of version db")
@click.option("--version", help="version of the value, optional")
@click.argument("store-key")
@click.argument("hex-key")
def get(dbpath, version, store_key, hex_key):
    """
    get a value at version
    """
    key = f"s/k:{store_key}/".decode() + binascii.unhexlify(hex_key)
    plain_db = rocksdb.DB(dbpath + "plain.db", rocksdb.Options())
    if version is None:
        v = plain_db.get(key)
    else:
        version = int(version)
    print(binascii.hexlify(v))

    history_db = rocksdb.DB(dbpath + "history.db", rocksdb.Options())
    bz = history_db.get(key)
    bm = BitMap64.deserialize(bz)

    # seek in bitmap
    bm.Rank(version)


@cli.command()
def sync(path):
    pass


@cli.command()
@click.option("--dbpath", help="path of rocksdb")
def rocksdbstats(dbpath):
    rocksdb_stats(Path(dbpath))


if __name__ == "__main__":
    cli()
