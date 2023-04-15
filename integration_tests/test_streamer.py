import pytest
from cprotobuf import Field, ProtoEntity, decode_primitive
from hexbytes import HexBytes

from .utils import ADDRS


class StoreKVPairs(ProtoEntity):
    # the store key for the KVStore this pair originates from
    store_key = Field("string", 1)
    # true indicates a delete operation
    delete = Field("bool", 2)
    key = Field("bytes", 3)
    value = Field("bytes", 4)


def decode_stream_file(data, entry_cls=StoreKVPairs):
    """
    StoreKVPairs, StoreKVPairs, ...
    """
    assert int.from_bytes(data[:8], "big") + 8 == len(data), "incomplete file"

    items = []
    offset = 8
    while offset < len(data):
        size, n = decode_primitive(data[offset:], "uint64")
        offset += n
        item = entry_cls()
        item.ParseFromString(data[offset : offset + size])
        items.append(item)
        offset += size
    return items


@pytest.mark.skip(reason="file streamer is not useful for now")
def test_streamers(cronos):
    """
    - check the streaming files are created
    - try to parse the state change sets
    """
    # inspect the first state change of the first tx in genesis
    # the InitChainer is committed together with the first block.
    path = cronos.node_home(0) / "data/file_streamer/block-1-data"
    items = decode_stream_file(open(path, "rb").read())
    # creation of the validator account
    assert items[0].store_key == "acc"
    # the writes are sorted by key, find the minimal address
    min_addr = min(ADDRS.values())
    assert items[0].key == b"\x01" + HexBytes(min_addr)


if __name__ == "__main__":
    import binascii
    import sys

    items = decode_stream_file(open(sys.argv[1], "rb").read())
    for item in items:
        print(
            item.store_key,
            item.delete,
            binascii.hexlify(item.key).decode(),
            binascii.hexlify(item.value).decode(),
        )
