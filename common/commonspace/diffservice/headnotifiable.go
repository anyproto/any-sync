package diffservice

type HeadNotifiable interface {
	UpdateHeads(id string, heads []string)
}

type HeadNotifiableFunc func(id string, heads []string)

func (h HeadNotifiableFunc) UpdateHeads(id string, heads []string) {
	h(id, heads)
}
