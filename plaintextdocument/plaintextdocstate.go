package plaintextdocument

import (
	"fmt"

	"github.com/anytypeio/go-anytype-infrastructure-experiments/testutils/testchanges/pb"
	"github.com/gogo/protobuf/proto"
)

type DocumentState struct {
	LastChangeId string
	Text         string
}

func NewDocumentState(text string, id string) *DocumentState {
	return &DocumentState{
		LastChangeId: id,
		Text:         text,
	}
}

func BuildDocumentStateFromChange(change []byte, id string) (*DocumentState, error) {
	var changesData pb.PlainTextChangeData
	err := proto.Unmarshal(change, &changesData)
	if err != nil {
		return nil, err
	}

	if changesData.GetSnapshot() == nil {
		return nil, fmt.Errorf("could not create state from empty snapshot")
	}
	return NewDocumentState(changesData.GetSnapshot().GetText(), id), nil
}

func (p *DocumentState) ApplyChange(change []byte, id string) (*DocumentState, error) {
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

func (p *DocumentState) applyChange(ch *pb.PlainTextChangeContent) error {
	switch {
	case ch.GetTextAppend() != nil:
		text := ch.GetTextAppend().GetText()
		p.Text += "|" + text
	}
	return nil
}
