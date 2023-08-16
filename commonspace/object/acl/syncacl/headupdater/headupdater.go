package headupdater

type HeadUpdater interface {
	UpdateHeads(id string, heads []string)
}
