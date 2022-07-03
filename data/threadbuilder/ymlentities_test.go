package threadbuilder

import (
	"fmt"
	"testing"
)

func Test_YamlParse(t *testing.T) {
	tb, _ := NewThreadBuilderFromFile("validsnapshotexample.yml")
	gr, _ := tb.Graph()
	fmt.Println(gr)
}
