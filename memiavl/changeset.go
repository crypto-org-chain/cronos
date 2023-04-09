package memiavl

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/cosmos/iavl"
)

// MarshalChangeSet encode change set to bytes
//
// ```
// delete: int8
// keyLen: varint-uint64
// key
// [ // if delete is false
//
//	valueLen: varint-uint64
//	value
//
// ]
// ```
func MarshalChangeSet(cs iavl.ChangeSet) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if err := MarshalChangeSetTo(cs, buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func MarshalChangeSetTo(cs iavl.ChangeSet, w io.Writer) error {
	for _, pair := range cs.Pairs {
		if _, err := w.Write([]byte{bool2byte(pair.Delete)}); err != nil {
			return err
		}

		if err := writeBytes(w, pair.Key); err != nil {
			return err
		}

		if !pair.Delete {
			if err := writeBytes(w, pair.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func UnmarshalChangeSet(data []byte) (iavl.ChangeSet, error) {
	var offset int
	var cs iavl.ChangeSet
	for offset < len(data) {
		pair, n := readKVPair(data[offset:])
		if n <= 0 {
			return iavl.ChangeSet{}, fmt.Errorf("decode kv pair failed: %d", n)
		}
		offset += n
		cs.Pairs = append(cs.Pairs, pair)
	}
	return cs, nil
}

func bool2byte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func writeBytes(w io.Writer, payload []byte) error {
	var numBuf [binary.MaxVarintLen64]byte

	n := binary.PutUvarint(numBuf[:], uint64(len(payload)))
	if _, err := w.Write(numBuf[:n]); err != nil {
		return err
	}
	if _, err := w.Write(payload); err != nil {
		return err
	}
	return nil
}

// readKVPair decode a key-value pair from reader
//
//	n == 0: buf too small
//	n  < 0: value larger than 64 bits (overflow)
func readKVPair(buf []byte) (*iavl.KVPair, int) {
	if len(buf) == 0 {
		return nil, 0
	}

	deletion := buf[0]
	offset := 1

	keyLen, n := binary.Uvarint(buf[offset:])
	if n <= 0 {
		return nil, n
	}
	offset += n

	if len(buf) < offset+int(keyLen) {
		return nil, 0
	}
	pair := iavl.KVPair{
		Delete: deletion == 1,
		Key:    buf[offset : offset+int(keyLen)],
	}
	offset += int(keyLen)

	if pair.Delete {
		return &pair, offset
	}

	valueLen, n := binary.Uvarint(buf[offset:])
	if n <= 0 {
		return nil, n
	}
	offset += n

	if len(buf) < offset+int(valueLen) {
		return nil, 0
	}
	pair.Value = buf[offset : offset+int(valueLen)]
	offset += int(valueLen)

	return &pair, offset
}
