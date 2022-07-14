package treestoragebuilder

import (
	"fmt"
	"testing"
)

func Test_YamlParse(t *testing.T) {
	tb, _ := NewTreeStorageBuilderWithTestName("userjoinexampleupdate.yml")
	gr, _ := tb.Graph()
	fmt.Println(gr)
}
