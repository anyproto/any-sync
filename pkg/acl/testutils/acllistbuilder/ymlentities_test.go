package acllistbuilder

import (
	"fmt"
	"testing"
)

func Test_YamlParse(t *testing.T) {
	tb, _ := NewListStorageWithTestName("userjoinexampleupdate.yml")
	gr, _ := tb.Graph()
	fmt.Println(gr)
}
