package exampledocument

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/data/pb"
	"github.com/gogo/protobuf/proto"
)

// TestDocumentState -> testutils
// ThreadBuilder -> testutils
// move protos to test utils

type PlainTextDocumentState struct {
	LastChangeId string
	Text         string
}

func NewPlainTextDocumentState(text string, id string) *PlainTextDocumentState {
	return &PlainTextDocumentState{
		LastChangeId: id,
		Text:         text,
	}
}

func (p *PlainTextDocumentState) ApplyChange(change []byte, id string) (DocumentState, error) {
	var changesData pb.PlainTextChangeData
	err := proto.Unmarshal(change, &changesData)
	if err != nil {
		return nil, err
	}

	for _, content := range changesData.GetContent() {
		err = p.applyChange(content)
		if err != nil {
			return nil, err
		}
	}
	p.LastChangeId = id
	return p, nil
}

func (p *PlainTextDocumentState) applyChange(ch *pb.PlainTextChangeContent) error {
	switch {
	case ch.GetTextAppend() != nil:
		text := ch.GetTextAppend().GetText()
		p.Text += "|" + text
	}
	return nil
}

type PlainTextDocumentStateProvider struct{}

func NewPlainTextDocumentStateProvider() *PlainTextDocumentStateProvider {
	return &PlainTextDocumentStateProvider{}
}

func (p *PlainTextDocumentStateProvider) ProvideFromInitialChange(change []byte, id string) (DocumentState, error) {
	var changesData pb.PlainTextChangeData
	err := proto.Unmarshal(change, &changesData)
	if err != nil {
		return nil, err
	}

	if changesData.GetSnapshot() == nil {
		return nil, fmt.Errorf("could not create state from empty snapshot")
	}
	return NewPlainTextDocumentState(changesData.GetSnapshot().GetText(), id), nil
}
