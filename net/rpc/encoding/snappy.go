package encoding

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/snappy"
	"storj.io/drpc"

	"github.com/anyproto/any-sync/protobuf"
)

var (
	rawBytes        atomic.Uint64
	compressedBytes atomic.Uint64
)

func init() {
	go func() {
		for {
			time.Sleep(10 * time.Second)
			fmt.Printf("rawBytes: %d; compressedBytes: %d\n", rawBytes.Load(), compressedBytes.Load())
		}
	}()
}

type snappyBuf struct {
	buf []byte
}

var snappyBytesPool = sync.Pool{
	New: func() any {
		return &snappyBuf{}
	},
}

type snappyEncoding struct{}

func (se snappyEncoding) Marshal(msg drpc.Message) ([]byte, error) {
	return se.MarshalAppend(nil, msg)
}

func (se snappyEncoding) MarshalAppend(buf []byte, msg drpc.Message) (res []byte, err error) {
	protoMessage, ok := msg.(protobuf.Message)
	if !ok {
		if protoMessageGettable, ok := msg.(ProtoMessageGettable); ok {
			protoMessage, err = protoMessageGettable.ProtoMessage()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, ErrNotAProtoMessage
		}
	}

	var marshalBuf *snappyBuf
	mBufPool := snappyBytesPool.Get()
	if mBufPool != nil {
		marshalBuf = mBufPool.(*snappyBuf)
	} else {
		marshalBuf = &snappyBuf{}
	}

	var prevLen = len(buf)
	if marshalBuf.buf, err = protobuf.MarshalAppend(marshalBuf.buf[:0], protoMessage); err != nil {
		return nil, err
	}
	msgData := marshalBuf.buf
	maxEncodedLen := snappy.MaxEncodedLen(len(msgData))
	bufSize := maxEncodedLen + prevLen
	buf = slices.Grow(buf, bufSize)[:bufSize]
	res = snappy.Encode(buf[prevLen:], msgData)
	rawBytes.Add(uint64(len(msgData)))
	compressedBytes.Add(uint64(len(res)))
	snappyBytesPool.Put(marshalBuf)
	return buf[:prevLen+len(res)], nil
}

func (se snappyEncoding) Unmarshal(buf []byte, msg drpc.Message) (err error) {
	decodedLen, err := snappy.DecodedLen(buf)
	if err != nil {
		return
	}

	var unmarshalBuf *snappyBuf
	mBufPool := snappyBytesPool.Get()
	if mBufPool != nil {
		unmarshalBuf = mBufPool.(*snappyBuf)
	} else {
		unmarshalBuf = &snappyBuf{}
	}

	unmarshalBuf.buf = slices.Grow(unmarshalBuf.buf, decodedLen)[:decodedLen]
	if unmarshalBuf.buf, err = snappy.Decode(unmarshalBuf.buf, buf); err != nil {
		return
	}

	compressedBytes.Add(uint64(len(buf)))
	rawBytes.Add(uint64(len(unmarshalBuf.buf)))

	var protoMessageSettable ProtoMessageSettable
	protoMessage, ok := msg.(protobuf.Message)
	if !ok {
		if protoMessageSettable, ok = msg.(ProtoMessageSettable); ok {
			protoMessage, err = protoMessageSettable.ProtoMessage()
			if err != nil {
				return
			}
		} else {
			return ErrNotAProtoMessage
		}
	}
	err = protoMessage.UnmarshalVT(unmarshalBuf.buf)
	if err != nil {
		return err
	}
	if protoMessageSettable != nil {
		err = protoMessageSettable.SetProtoMessage(protoMessage)
	}
	snappyBytesPool.Put(unmarshalBuf)
	return
}
