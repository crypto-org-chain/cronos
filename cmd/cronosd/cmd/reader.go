package cmd

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/gogo/protobuf/proto"
)

type varintReader struct {
	r       io.ReadSeeker
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

func NewDelimitedReader(r io.ReadSeeker, maxSize int) *varintReader {
	var closer io.Closer
	if c, ok := r.(io.Closer); ok {
		closer = c
	}
	return &varintReader{r, nil, maxSize, closer}
}

func (r *varintReader) ReadNextLength(seekOnly bool) (int, int, []byte, error) {
	byteReader := newByteReader(r.r)
	l, err := binary.ReadUvarint(byteReader)
	n := byteReader.bytesRead
	if err != nil {
		return n, -1, nil, err
	}

	length := int(l)
	if l >= uint64(^uint(0)>>1) || length < 0 || n+length < 0 {
		return n, -1, nil, fmt.Errorf("invalid out-of-range message length %v", l)
	}
	if length > r.maxSize {
		return n, -1, nil, fmt.Errorf("message exceeds max size (%v > %v)", length, r.maxSize)
	}
	now := time.Now()
	var buf []byte
	if seekOnly {
		_, err = r.r.Seek(int64(length), 1)
	} else {
		buf = make([]byte, length)
		_, err = io.ReadFull(r.r, buf)
	}
	log.Println("seek:", time.Since(now), length)
	return n, length, buf, err
}

func (r *varintReader) ReadMsgWithLength(length int, msg proto.Message) (int, error) {
	if len(r.buf) < length {
		r.buf = make([]byte, length)
	}
	now := time.Now()
	buf := r.buf[:length]
	nr, err := io.ReadFull(r.r, buf)
	now2 := time.Now()
	log.Println("read:", now2.Sub(now), length)
	if err != nil {
		return nr, err
	}
	err = proto.Unmarshal(buf, msg)
	log.Println("unmarshal:", time.Since(now2))
	return nr, err
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
	now := time.Now()
	buf := r.buf[:length]
	nr, err := io.ReadFull(r.r, buf)
	now2 := time.Now()
	log.Println("read:", now2.Sub(now), length)
	n += nr
	if err != nil {
		return n, err
	}
	err = proto.Unmarshal(buf, msg)
	log.Println("unmarshal:", time.Since(now2))
	return n, err
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
