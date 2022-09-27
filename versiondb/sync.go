package versiondb

import (
	"bufio"
	"io"

	protoio "github.com/gogo/protobuf/io"

	"github.com/cosmos/cosmos-sdk/store/types"
)

const maxItemSize = 64000000 // SDK has no key/value size limit, so we set an arbitrary limit

// ReadFileStreamer parse a binary stream dumped by file streamer to changeset,
// which can be feeded to version store.
func ReadFileStreamer(input *bufio.Reader) ([]types.StoreKVPair, error) {
	var changeSet []types.StoreKVPair
	reader := protoio.NewDelimitedReader(input, maxItemSize)
	for {
		var msg types.StoreKVPair
		err := reader.ReadMsg(&msg)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		changeSet = append(changeSet, msg)
	}
	return changeSet, nil
}
