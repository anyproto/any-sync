package objecttree

type DescriptionParser interface {
	ParseChange(ch *Change, isRoot bool) ([]string, error)
}

var NoOpDescriptionParser = noopDescriptionParser{}

type noopDescriptionParser struct{}

func (n noopDescriptionParser) ParseChange(ch *Change, isRoot bool) ([]string, error) {
	return []string{"DOC"}, nil
}
