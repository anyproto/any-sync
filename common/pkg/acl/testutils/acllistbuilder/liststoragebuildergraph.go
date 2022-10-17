//go:build ((!linux && !darwin) || android || ios || nographviz) && !amd64
// +build !linux,!darwin android ios nographviz
// +build !amd64

package acllistbuilder

import "fmt"

func (t *ACLListStorageBuilder) Graph() (string, error) {
	return "", fmt.Errorf("building graphs is not supported")
}
