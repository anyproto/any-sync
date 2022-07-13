//go:build ((!linux && !darwin) || android || ios || nographviz) && !amd64
// +build !linux,!darwin android ios nographviz
// +build !amd64

package acltree

import "fmt"

func (t *Tree) Graph() (data string, err error) {
	return "", fmt.Errorf("not supported")
}
