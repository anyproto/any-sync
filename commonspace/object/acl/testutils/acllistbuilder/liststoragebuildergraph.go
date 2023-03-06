//go:build ((!linux && !darwin) || android || ios || nographviz || !cgo) && !amd64
// +build !linux,!darwin android ios nographviz !cgo
// +build !amd64

package acllistbuilder

import "fmt"

func (t *AclListStorageBuilder) Graph() (string, error) {
	return "", fmt.Errorf("building graphs is not supported")
}
