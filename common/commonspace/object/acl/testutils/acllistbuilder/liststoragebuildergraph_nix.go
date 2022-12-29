//go:build (linux || darwin) && !android && !ios && !nographviz && (amd64 || arm64)
// +build linux darwin
// +build !android
// +build !ios
// +build !nographviz
// +build amd64 arm64

package acllistbuilder

import (
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/commonspace/object/acl/aclrecordproto"
	"github.com/gogo/protobuf/proto"
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

func (t *AclListStorageBuilder) Graph() (string, error) {
	// TODO: check updates on https://github.com/goccy/go-graphviz/issues/52 or make a fix yourself to use better library here
	graph := gographviz.NewGraph()
	graph.SetName("G")
	graph.SetDir(true)
	var nodes = make(map[string]struct{})

	var addNodes = func(r *aclrecordproto.AclRecord, id string) error {
		style := "solid"

		var chSymbs []string
		aclData := &aclrecordproto.AclData{}
		err := proto.Unmarshal(r.GetData(), aclData)
		if err != nil {
			return err
		}

		for _, chc := range aclData.AclContent {
			tp := fmt.Sprintf("%T", chc.Value)
			tp = strings.Replace(tp, "AclChangeAclContentValueValueOf", "", 1)
			res := ""
			for _, ts := range tp {
				if unicode.IsUpper(ts) {
					res += string(ts)
				}
			}
			chSymbs = append(chSymbs, res)
		}

		shortId := id
		label := fmt.Sprintf("Id: %s\nChanges: %s\n",
			shortId,
			strings.Join(chSymbs, ","),
		)
		e := graph.AddNode("G", "\""+id+"\"", map[string]string{
			"label": "\"" + label + "\"",
			"style": "\"" + style + "\"",
		})
		if e != nil {
			return e
		}
		nodes[id] = struct{}{}
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

	var addLinks = func(r *aclrecordproto.AclRecord, id string) error {
		if r.PrevId == "" {
			return nil
		}
		err := createEdge(id, r.PrevId, EdgeParameters{
			style: "dashed",
			color: "red",
		})
		if err != nil {
			return err
		}

		return nil
	}

	err := t.traverseFromHead(addNodes)
	if err != nil {
		return "", err
	}

	err = t.traverseFromHead(addLinks)
	if err != nil {
		return "", err
	}

	return graph.String(), nil
}
