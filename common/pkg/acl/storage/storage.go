package storage

type Storage interface {
	ID() (string, error)
}
