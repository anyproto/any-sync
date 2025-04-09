package protobuf

import (
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"
)

type Message interface {
	SizeVT() (n int)
	MarshalVT() (dAtA []byte, err error)
	UnmarshalVT(dAtA []byte) error
	MarshalToSizedBufferVT(dAtA []byte) (int, error)
}

func MarshalAppend(buf []byte, pb proto.Message) ([]byte, error) {
	if m, ok := pb.(Message); ok {
		siz := m.SizeVT()
		offset := len(buf)
		buf = slices.Grow(buf, offset+siz)[:offset+siz]
		return MarshalToSizedBuffer(m, buf, offset)
	}
	return nil, fmt.Errorf("proto: MarshalAppend not supported by %T", pb)
}

func MarshalToSizedBuffer(m Message, b []byte, offset int) ([]byte, error) {
	_, err := m.MarshalToSizedBufferVT(b[offset:])
	if err != nil {
		return nil, err
	}
	return b, nil
}
