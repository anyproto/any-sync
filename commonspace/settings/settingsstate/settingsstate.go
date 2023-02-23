package settingsstate

type State struct {
	DeletedIds     []string
	DeleterId      string
	LastIteratedId string
}
