package diffservice

type HeadNotifiable interface {
	UpdateHeads(id string, heads []string)
}
