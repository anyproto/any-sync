package threadbuilder

import (
	"fmt"
	"testing"
)

func Test_YamlParse(t *testing.T) {
	tb, _ := NewThreadBuilderFromFile("userjoinexample.yml")
	gr, _ := tb.Graph()
	fmt.Println(gr)
}
