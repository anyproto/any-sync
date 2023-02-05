//go:build (!linux && !darwin) || android || ios || nographviz || windows
// +build !linux,!darwin android ios nographviz windows

package objecttree

import "fmt"

func (t *Tree) Graph(parser DescriptionParser) (data string, err error) {
	return "", fmt.Errorf("not supported")
}
