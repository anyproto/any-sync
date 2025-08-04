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
	proto.Message
}

func MarshalAppend(buf []byte, pb proto.Message) ([]byte, error) {
	if m, ok := pb.(Message); ok {
		siz := m.SizeVT()
		offset := len(buf)
		buf = slices.Grow(buf, offset+siz)[:offset+siz]
		_, err := m.MarshalToSizedBufferVT(buf[offset:])
		if err != nil {
			return nil, err
		}
		return buf, nil
	}
	return nil, fmt.Errorf("proto: MarshalAppend not supported by %T", pb)
}
