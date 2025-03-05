package protobuf

import (
	"fmt"
	"google.golang.org/protobuf/proto"
)

type ProtoBuf interface {
	SizeVT() (n int)
	MarshalVT() (dAtA []byte, err error)
	UnmarshalVT(dAtA []byte) error
	MarshalToSizedBufferVT(dAtA []byte) (int, error)
}

func MarshalAppend(buf []byte, pb proto.Message) ([]byte, error) {
	if m, ok := pb.(ProtoBuf); ok {
		siz := m.SizeVT()
		if cap(buf) < len(buf)+siz {
			newBuf := make([]byte, 0, len(buf)+siz)
			buf = append(newBuf, buf...)
		}
		return MarshalToSizedBuffer(m, buf, len(buf)+siz)
	}
	return nil, fmt.Errorf("proto: MarshalAppend not supported by %T", pb)
}

func MarshalToSizedBuffer(m ProtoBuf, b []byte, newLen int) ([]byte, error) {
	b = b[:newLen]
	_, err := m.MarshalToSizedBufferVT(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
