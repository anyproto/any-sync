package objecttree

type DescriptionParser interface {
	ParseChange(*Change) ([]string, error)
}

var NoOpDescriptionParser = noopDescriptionParser{}

type noopDescriptionParser struct{}

func (n noopDescriptionParser) ParseChange(change *Change) ([]string, error) {
	return []string{"DOC"}, nil
}
