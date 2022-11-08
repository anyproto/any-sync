//go:build (linux || darwin) && !android && !ios && !nographviz && (amd64 || arm64)
// +build linux darwin
// +build !android
// +build !ios
// +build !nographviz
// +build amd64 arm64

package tree

import (
	"bytes"
	"fmt"
	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"strings"
	"time"
)

func (t *Tree) Graph(parser DescriptionParser) (data string, err error) {
	var order = make(map[string]string)
	var seq = 0
	t.Iterate(t.RootId(), func(c *Change) (isContinue bool) {
		v := order[c.Id]
		if v == "" {
			order[c.Id] = fmt.Sprint(seq)
		} else {
			order[c.Id] = fmt.Sprintf("%s,%d", v, seq)
		}
		seq++
		return true
	})
	g := graphviz.New()
	defer g.Close()
	graph, err := g.Graph()
	if err != nil {
		return
	}
	defer func() {
		err = graph.Close()
	}()
	var nodes = make(map[string]*cgraph.Node)
	var addChange = func(c *Change) error {
		n, e := graph.CreateNode(c.Id)
		if e != nil {
			return e
		}
		n.SetStyle(cgraph.FilledNodeStyle)
		nodes[c.Id] = n
		ord := order[c.Id]
		if ord == "" {
			ord = "miss"
		}
		chSymbs, err := parser.ParseChange(c)
		if err != nil {
			return err
		}

		shortId := c.Id
		label := fmt.Sprintf("Id: %s\nOrd: %s\nTime: %s\nChanges: %s\n",
			shortId,
			ord,
			time.Unix(c.Timestamp, 0).Format("02.01.06 15:04:05"),
			strings.Join(chSymbs, ","),
		)
		n.SetLabel(label)
		return nil
	}
	for _, c := range t.attached {
		if err = addChange(c); err != nil {
			return
		}
	}
	for _, c := range t.unAttached {
		if err = addChange(c); err != nil {
			return
		}
	}
	var getNode = func(id string) (*cgraph.Node, error) {
		if n, ok := nodes[id]; ok {
			return n, nil
		}
		n, err := graph.CreateNode(fmt.Sprintf("%s: not in Tree", id))
		if err != nil {
			return nil, err
		}
		nodes[id] = n
		return n, nil
	}
	var addLinks = func(c *Change) error {
		for _, prevId := range c.PreviousIds {
			self, e := getNode(c.Id)
			if e != nil {
				return e
			}
			prev, e := getNode(prevId)
			if e != nil {
				return e
			}
			_, e = graph.CreateEdge("", self, prev)
			if e != nil {
				return e
			}
		}
		return nil
	}
	for _, c := range t.attached {
		if err = addLinks(c); err != nil {
			return
		}
	}
	for _, c := range t.unAttached {
		if err = addLinks(c); err != nil {
			return
		}
	}
	var buf bytes.Buffer
	if err = g.Render(graph, "dot", &buf); err != nil {
		return
	}
	return buf.String(), nil
}
