package syncdeps

type Message interface {
	ObjectId() string
	MsgSize() uint64
}
