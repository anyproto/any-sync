package db

type Tree interface {
	Id() string
	UpdateHead(head string) (err error)
	AddChange(key string, value []byte) (err error)
}

type Space interface {
	Id() string
	CreateTree(id string) (Tree, error)
	GetTree(id string) (Tree, error)
	Close() error
}

type SpaceCreator interface {
	CreateSpace(id string) (Space, error)
	GetSpace(id string) (Space, error)
	Close() error
}

type SpaceCreatorFactory func() SpaceCreator
