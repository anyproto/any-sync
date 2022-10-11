package db

import (
	"fmt"
	"github.com/akrylysov/pogreb"
	"path"
)

type pogrebTree struct {
	id string
	db *pogreb.DB
}

func (p *pogrebTree) Id() string {
	return p.id
}

func (p *pogrebTree) UpdateHead(head string) (err error) {
	return p.db.Put([]byte(fmt.Sprintf("t/%s/heads", p.id)), []byte(head))
}

func (p *pogrebTree) AddChange(key string, value []byte) (err error) {
	return p.db.Put([]byte(fmt.Sprintf("t/%s/%s", p.id, key)), value)
}

type pogrebSpace struct {
	id string
	db *pogreb.DB
}

func (p *pogrebSpace) Id() string {
	return p.id
}

func (p *pogrebSpace) CreateTree(id string) (Tree, error) {
	return &pogrebTree{
		id: id,
		db: p.db,
	}, nil
}

func (p *pogrebSpace) GetTree(id string) (Tree, error) {
	return p.CreateTree(id)
}

func (p *pogrebSpace) Close() error {
	return p.db.Close()
}

type pogrebSpaceCreator struct {
	rootPath string
}

func (p *pogrebSpaceCreator) CreateSpace(id string) (Space, error) {
	dbPath := path.Join(p.rootPath, id)
	db, err := pogreb.Open(dbPath, &pogreb.Options{
		BackgroundSyncInterval:       0,
		BackgroundCompactionInterval: 20000,
	})
	if err != nil {
		return nil, err
	}
	return &pogrebSpace{
		id: id,
		db: db,
	}, nil
}

func (p *pogrebSpaceCreator) GetSpace(id string) (Space, error) {
	return p.CreateSpace(id)
}

func (p *pogrebSpaceCreator) Close() error {
	return nil
}

func NewPogrebSpaceCreator() SpaceCreator {
	return &pogrebSpaceCreator{rootPath: "db.test"}
}
