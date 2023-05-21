package settingsstate

import "golang.org/x/exp/slices"

type State struct {
	DeletedIds     []string
	DeleterId      string
	LastIteratedId string
}

func (s *State) Exists(id string) bool {
	// using map here will not give a lot of benefit, because this thing should be called only
	// when we are adding content, thus it doesn't matter
	return slices.Contains(s.DeletedIds, id)
}
