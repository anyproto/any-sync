package data

type DocumentState interface {
	ApplyChange(change []byte, id string) (DocumentState, error)
}

type InitialStateProvider interface {
	ProvideFromInitialChange(change []byte, id string) (DocumentState, error)
}
