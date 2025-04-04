package encoding

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anyproto/protobuf/proto"
	"github.com/golang/snappy"
	"storj.io/drpc"
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

var snappyPool = sync.Pool{
	New: func() any {
		return &snappyEncoding{}
	},
}

type snappyEncoding struct {
	marshalBuf   []byte
	unmarshalBuf []byte
}

func (se *snappyEncoding) Marshal(msg drpc.Message) ([]byte, error) {
	return se.MarshalAppend(nil, msg)
}

func (se *snappyEncoding) MarshalAppend(buf []byte, msg drpc.Message) (res []byte, err error) {
	protoMessage, ok := msg.(proto.Message)
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
	if se.marshalBuf, err = proto.MarshalAppend(se.marshalBuf[:0], protoMessage); err != nil {
		return nil, err
	}

	maxEncodedLen := snappy.MaxEncodedLen(len(se.marshalBuf))
	buf = slices.Grow(buf, maxEncodedLen)[:maxEncodedLen]
	res = snappy.Encode(buf, se.marshalBuf)

	rawBytes.Add(uint64(len(se.marshalBuf)))
	compressedBytes.Add(uint64(len(res)))
	return res, nil
}

func (se *snappyEncoding) Unmarshal(buf []byte, msg drpc.Message) (err error) {
	decodedLen, err := snappy.DecodedLen(buf)
	if err != nil {
		return
	}
	se.unmarshalBuf = slices.Grow(se.unmarshalBuf, decodedLen)[:decodedLen]
	if se.unmarshalBuf, err = snappy.Decode(se.unmarshalBuf, buf); err != nil {
		return
	}

	compressedBytes.Add(uint64(len(buf)))
	rawBytes.Add(uint64(len(se.unmarshalBuf)))

	var protoMessageSettable ProtoMessageSettable
	protoMessage, ok := msg.(proto.Message)
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
	err = proto.Unmarshal(se.unmarshalBuf, protoMessage)
	if err != nil {
		return err
	}
	if protoMessageSettable != nil {
		err = protoMessageSettable.SetProtoMessage(protoMessage)
	}
	return
}
