//go:build (linux || darwin) && !android && !ios && !nographviz && (amd64 || arm64)
// +build linux darwin
// +build !android
// +build !ios
// +build !nographviz
// +build amd64 arm64

package threadbuilder

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/awalterschulze/gographviz"
)

// To quickly look at visualized string you can use https://dreampuf.github.io/GraphvizOnline

type EdgeParameters struct {
	style string
	color string
	label string
}

func (t *ThreadBuilder) Graph() (string, error) {
	// TODO: check updates on https://github.com/goccy/go-graphviz/issues/52 or make a fix yourself to use better library here
	graph := gographviz.NewGraph()
	graph.SetName("G")
	graph.SetDir(true)
	var nodes = make(map[string]struct{})

	var addNodes = func(r *threadChange) error {
		// TODO: revisit function after checking

		style := "solid"
		if r.GetAclData() != nil {
			style = "filled"
		} else if r.changesData != nil {
			style = "dashed"
		}

		var chSymbs []string
		if r.changesData != nil {
			for _, chc := range r.changesData.Content {
				tp := fmt.Sprintf("%T", chc.Value)
				tp = strings.Replace(tp, "ChangeContentValueOf", "", 1)
				res := ""
				for _, ts := range tp {
					if unicode.IsUpper(ts) {
						res += string(ts)
					}
				}
				chSymbs = append(chSymbs, res)
			}
		}
		if r.GetAclData() != nil {
			for _, chc := range r.GetAclData().AclContent {
				tp := fmt.Sprintf("%T", chc.Value)
				tp = strings.Replace(tp, "ACLChangeACLContentValueValueOf", "", 1)
				res := ""
				for _, ts := range tp {
					if unicode.IsUpper(ts) {
						res += string(ts)
					}
				}
				chSymbs = append(chSymbs, res)
			}
		}

		shortId := r.id
		label := fmt.Sprintf("Id: %s\nChanges: %s\n",
			shortId,
			strings.Join(chSymbs, ","),
		)
		e := graph.AddNode("G", "\""+r.id+"\"", map[string]string{
			"label": "\"" + label + "\"",
			"style": "\"" + style + "\"",
		})
		if e != nil {
			return e
		}
		nodes[r.id] = struct{}{}
		return nil
	}

	var createEdge = func(firstId, secondId string, params EdgeParameters) error {
		_, exists := nodes[firstId]
		if !exists {
			return fmt.Errorf("no such node")
		}
		_, exists = nodes[secondId]
		if !exists {
			return fmt.Errorf("no previous node")
		}

		err := graph.AddEdge("\""+firstId+"\"", "\""+secondId+"\"", true, map[string]string{
			"color": params.color,
			"style": params.style,
		})
		if err != nil {
			return err
		}

		return nil
	}

	var addLinks = func(t *threadChange) error {
		for _, prevId := range t.AclHeadIds {
			err := createEdge(t.id, prevId, EdgeParameters{
				style: "dashed",
				color: "red",
			})
			if err != nil {
				return err
			}
		}

		for _, prevId := range t.TreeHeadIds {
			err := createEdge(t.id, prevId, EdgeParameters{
				style: "dashed",
				color: "blue",
			})
			if err != nil {
				return err
			}
		}

		if t.SnapshotBaseId != "" {
			err := createEdge(t.id, t.SnapshotBaseId, EdgeParameters{
				style: "bold",
				color: "blue",
			})
			if err != nil {
				return err
			}
		}

		return nil
	}

	err := t.traverseFromHeads(addNodes)
	if err != nil {
		return "", err
	}

	err = t.traverseFromHeads(addLinks)
	if err != nil {
		return "", err
	}

	return graph.String(), nil
}
