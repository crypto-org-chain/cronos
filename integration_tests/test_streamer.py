from cprotobuf import Field, ProtoEntity, decode_primitive
from hexbytes import HexBytes

from .utils import ADDRS


class StoreKVPairs(ProtoEntity):
    # the store key for the KVStore this pair originates from
    store_key = Field("string", 1)
    # true indicates a set operation, false indicates a delete operation
    delete = Field("bool", 2)
    key = Field("bytes", 3)
    value = Field("bytes", 4)


def decode_stream_file(data, body_cls=StoreKVPairs, header_cls=None, footer_cls=None):
    """
    header, body*, footer
    """
    header = footer = None
    body = []
    offset = 0
    size, n = decode_primitive(data, "uint64")
    offset += n

    # header
    if header_cls is not None:
        header = header_cls()
        header.ParseFromString(data[offset : offset + size])
    offset += size

    while True:
        size, n = decode_primitive(data[offset:], "uint64")
        offset += n
        if offset + size == len(data):
            # footer
            if footer_cls is not None:
                footer = footer_cls()
                footer.ParseFromString(data[offset : offset + size])
            offset += size
            break
        else:
            # body
            if body_cls is not None:
                item = body_cls()
                item.ParseFromString(data[offset : offset + size])
                body.append(item)
            offset += size
    return header, body, footer


def test_streamers(cronos):
    """
    - check the streaming files are created
    - try to parse the state change sets
    """
    # inspect the first state change of the first tx in genesis
    path = cronos.node_home(0) / "data/file_streamer/block-0-tx-0"
    _, body, _ = decode_stream_file(open(path, "rb").read())
    # creation of the validator account
    assert body[0]["store_key"] == "acc"
    assert body[0]["key"] == b"\x01" + HexBytes(ADDRS["validator"])
