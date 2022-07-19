package cmd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/libs/protoio"
)

type varintReader struct {
	r       io.Reader
	buf     []byte
	maxSize int
	closer  io.Closer
}

type byteReader struct {
	reader    io.Reader
	buf       []byte
	bytesRead int
}

func newByteReader(r io.Reader) *byteReader {
	return &byteReader{
		reader: r,
		buf:    make([]byte, 1),
	}
}

func (r *byteReader) ReadByte() (byte, error) {
	n, err := r.reader.Read(r.buf)
	r.bytesRead += n
	if err != nil {
		return 0x00, err
	}
	return r.buf[0], nil
}

func NewDelimitedReader(r io.Reader, maxSize int) protoio.ReadCloser {
	var closer io.Closer
	if c, ok := r.(io.Closer); ok {
		closer = c
	}
	return &varintReader{r, nil, maxSize, closer}
}

func (r *varintReader) ReadMsg(msg proto.Message) (int, error) {
	byteReader := newByteReader(r.r)
	l, err := binary.ReadUvarint(byteReader)
	n := byteReader.bytesRead
	if err != nil {
		return n, err
	}

	length := int(l)
	if l >= uint64(^uint(0)>>1) || length < 0 || n+length < 0 {
		return n, fmt.Errorf("invalid out-of-range message length %v", l)
	}
	if length > r.maxSize {
		return n, fmt.Errorf("message exceeds max size (%v > %v)", length, r.maxSize)
	}

	if len(r.buf) < length {
		r.buf = make([]byte, length)
	}
	buf := r.buf[:length]
	nr, err := io.ReadFull(r.r, buf)
	n += nr
	if err != nil {
		return n, err
	}
	return n, proto.Unmarshal(buf, msg)
}

func (r *varintReader) Close() error {
	if r.closer != nil {
		return r.closer.Close()
	}
	return nil
}

func UnmarshalDelimited(data []byte, msg proto.Message) error {
	_, err := NewDelimitedReader(bytes.NewReader(data), len(data)).ReadMsg(msg)
	return err
}
