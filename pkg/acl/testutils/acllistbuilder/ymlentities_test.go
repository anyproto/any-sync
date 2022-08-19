package acllistbuilder

import (
	"fmt"
	"testing"
)

func Test_YamlParse(t *testing.T) {
	tb, _ := NewACLListStorageBuilderWithTestName("userjoinexampleupdate.yml")
	gr, _ := tb.Graph()
	fmt.Println(gr)
}
