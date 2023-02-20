package settingsstate

import "time"

type State struct {
	DeletedIds        []string
	SpaceDeletionDate time.Time
	LastIteratedId    string
}
