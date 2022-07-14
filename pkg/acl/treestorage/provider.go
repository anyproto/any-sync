package treestorage

import "fmt"

type Provider interface {
	TreeStorage(treeId string) (TreeStorage, error)
	InsertTree(tree TreeStorage) error
}

type inMemoryTreeStorageProvider struct {
	trees map[string]TreeStorage
}

func (i *inMemoryTreeStorageProvider) TreeStorage(treeId string) (TreeStorage, error) {
	if tree, exists := i.trees[treeId]; exists {
		return tree, nil
	}
	return nil, fmt.Errorf("tree with id %s doesn't exist", treeId)
}

func (i *inMemoryTreeStorageProvider) InsertTree(tree TreeStorage) error {
	if tree == nil {
		return fmt.Errorf("tree should not be nil")
	}

	id, err := tree.TreeID()
	if err != nil {
		return err
	}

	i.trees[id] = tree
	return nil
}

func NewInMemoryTreeStorageProvider() Provider {
	return &inMemoryTreeStorageProvider{
		trees: make(map[string]TreeStorage),
	}
}
