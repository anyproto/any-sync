package db

type Transaction interface {
	AddChange(key string, value []byte) (err error)
	GetChange(key string) ([]byte, error)
}

type Tree interface {
	Id() string
	UpdateHead(head string) (err error)
	AddChange(key string, value []byte) (err error)
	GetChange(key string) ([]byte, error)
	HasChange(key string) (has bool, err error)
	Perform(func(txn Transaction) error) error
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
