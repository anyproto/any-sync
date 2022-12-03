package api

import (
	"context"
	"errors"
	"fmt"
	clientproto "github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	nodeproto "github.com/anytypeio/go-anytype-infrastructure-experiments/node/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api/client"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api/node"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/peers"
	"strconv"
)

const CName = "debug.api"

var log = logger.NewNamed(CName)

var (
	ErrNoSuchCommand        = errors.New("no such command")
	ErrIncorrectPeerType    = errors.New("incorrect peer type")
	ErrIncorrectParamsCount = errors.New("incorrect params count")
)

type Command struct {
	Cmd func(server peers.Peer, params []string) (string, error)
}

type Script struct {
	Cmd func(params []string) (string, error)
}

type Service interface {
	app.Component
	Call(server peers.Peer, cmdName string, params []string) (string, error)
	Script(scriptName string, params []string) (res string, err error)
}

type service struct {
	clientCommands map[string]Command
	nodeCommands   map[string]Command
	scripts        map[string]Script
	client         client.Service
	node           node.Service
	peers          peers.Service
}

func New() Service {
	return &service{
		clientCommands: map[string]Command{},
		nodeCommands:   map[string]Command{},
		scripts:        map[string]Script{},
	}
}

func (s *service) Init(a *app.App) (err error) {
	s.client = a.MustComponent(client.CName).(client.Service)
	s.node = a.MustComponent(node.CName).(node.Service)
	s.peers = a.MustComponent(peers.CName).(peers.Service)
	s.registerClientCommands()
	s.registerNodeCommands()
	s.registerScripts()

	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Script(scriptName string, params []string) (res string, err error) {
	script, ok := s.scripts[scriptName]
	if !ok {
		err = ErrNoSuchCommand
		return
	}
	return script.Cmd(params)
}

func (s *service) Call(server peers.Peer, cmdName string, params []string) (res string, err error) {
	var (
		cmd      Command
		commands map[string]Command
	)
	switch server.PeerType {
	case peers.PeerTypeClient:
		commands = s.clientCommands
	case peers.PeerTypeNode:
		commands = s.nodeCommands
	}
	cmd, ok := commands[cmdName]
	if !ok {
		err = ErrNoSuchCommand
		return
	}
	return cmd.Cmd(server, params)
}

func (s *service) registerClientCommands() {
	s.clientCommands["create-space"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 0 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.client.CreateSpace(context.Background(), server.Address, &clientproto.CreateSpaceRequest{})
		if err != nil {
			return
		}
		res = resp.Id
		return
	}}
	s.clientCommands["derive-space"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 0 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.client.DeriveSpace(context.Background(), server.Address, &clientproto.DeriveSpaceRequest{})
		if err != nil {
			return
		}
		res = resp.Id
		return
	}}
	s.clientCommands["create-document"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 1 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.client.CreateDocument(context.Background(), server.Address, &clientproto.CreateDocumentRequest{
			SpaceId: params[0],
		})
		if err != nil {
			return
		}
		res = resp.Id
		return
	}}
	s.clientCommands["delete-document"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 2 {
			err = ErrIncorrectParamsCount
			return
		}
		_, err = s.client.DeleteDocument(context.Background(), server.Address, &clientproto.DeleteDocumentRequest{
			SpaceId:    params[0],
			DocumentId: params[1],
		})
		if err != nil {
			return
		}
		res = "deleted"
		return
	}}
	s.clientCommands["add-text"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 3 && len(params) != 4 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.client.AddText(context.Background(), server.Address, &clientproto.AddTextRequest{
			SpaceId:    params[0],
			DocumentId: params[1],
			Text:       params[2],
			IsSnapshot: len(params) == 4,
		})
		if err != nil {
			return
		}
		res = resp.DocumentId + "->" + resp.RootId + "->" + resp.HeadId
		return
	}}
	s.clientCommands["load-space"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 1 {
			err = ErrIncorrectParamsCount
			return
		}
		_, err = s.client.LoadSpace(context.Background(), server.Address, &clientproto.LoadSpaceRequest{
			SpaceId: params[0],
		})
		if err != nil {
			return
		}
		res = params[0]
		return
	}}
	s.clientCommands["all-trees"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 1 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.client.AllTrees(context.Background(), server.Address, &clientproto.AllTreesRequest{
			SpaceId: params[0],
		})
		if err != nil {
			return
		}
		for treeIdx, tree := range resp.Trees {
			treeStr := tree.Id + ":["
			for headIdx, head := range tree.Heads {
				treeStr += head
				if headIdx != len(tree.Heads)-1 {
					treeStr += ","
				}
			}
			treeStr += "]"
			res += treeStr
			if treeIdx != len(resp.Trees)-1 {
				res += "\n"
			}
		}
		return
	}}
	s.clientCommands["dump-tree"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 2 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.client.DumpTree(context.Background(), server.Address, &clientproto.DumpTreeRequest{
			SpaceId:    params[0],
			DocumentId: params[1],
		})
		if err != nil {
			return
		}
		res = resp.Dump
		return
	}}
	s.clientCommands["tree-params"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 2 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.client.TreeParams(context.Background(), server.Address, &clientproto.TreeParamsRequest{
			SpaceId:    params[0],
			DocumentId: params[1],
		})
		if err != nil {
			return
		}
		res = resp.RootId + "->"
		for headIdx, head := range resp.HeadIds {
			res += head
			if headIdx != len(resp.HeadIds)-1 {
				res += ","
			}
		}
		return
	}}
	s.clientCommands["all-spaces"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 0 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.client.AllSpaces(context.Background(), server.Address, &clientproto.AllSpacesRequest{})
		if err != nil {
			return
		}
		for treeIdx, spaceId := range resp.SpaceIds {
			res += spaceId
			if treeIdx != len(resp.SpaceIds)-1 {
				res += "\n"
			}
		}
		return
	}}
}

func (s *service) registerNodeCommands() {
	s.nodeCommands["all-trees"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 1 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.node.AllTrees(context.Background(), server.Address, &nodeproto.AllTreesRequest{
			SpaceId: params[0],
		})
		if err != nil {
			return
		}
		for treeIdx, tree := range resp.Trees {
			treeStr := tree.Id + ":["
			for headIdx, head := range tree.Heads {
				treeStr += head
				if headIdx != len(tree.Heads)-1 {
					treeStr += ","
				}
			}
			treeStr += "]"
			res += treeStr
			if treeIdx != len(resp.Trees)-1 {
				res += "\n"
			}
		}
		return
	}}
	s.nodeCommands["dump-tree"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 2 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.node.DumpTree(context.Background(), server.Address, &nodeproto.DumpTreeRequest{
			SpaceId:    params[0],
			DocumentId: params[1],
		})
		if err != nil {
			return
		}
		res = resp.Dump
		return
	}}
	s.nodeCommands["tree-params"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 2 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.node.TreeParams(context.Background(), server.Address, &nodeproto.TreeParamsRequest{
			SpaceId:    params[0],
			DocumentId: params[1],
		})
		if err != nil {
			return
		}
		res = resp.RootId + "->"
		for headIdx, head := range resp.HeadIds {
			res += head
			if headIdx != len(resp.HeadIds)-1 {
				res += ","
			}
		}
		return
	}}
	s.nodeCommands["all-spaces"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 0 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := s.node.AllSpaces(context.Background(), server.Address, &nodeproto.AllSpacesRequest{})
		if err != nil {
			return
		}
		for treeIdx, spaceId := range resp.SpaceIds {
			res += spaceId
			if treeIdx != len(resp.SpaceIds)-1 {
				res += "\n"
			}
		}
		return
	}}
}

func (s *service) registerScripts() {
	s.scripts["create-many"] = Script{Cmd: func(params []string) (res string, err error) {
		if len(params) != 5 {
			err = ErrIncorrectParamsCount
			return
		}
		peer, err := s.peers.Get(params[0])
		if err != nil {
			return
		}
		last, err := strconv.Atoi(params[4])
		if err != nil {
			return
		}
		if last <= 0 {
			err = fmt.Errorf("incorrect number of steps")
			return
		}

		for i := 0; i < last; i++ {
			_, err := s.client.AddText(context.Background(), peer.Address, &clientproto.AddTextRequest{
				SpaceId:    params[1],
				DocumentId: params[2],
				Text:       params[3],
			})
			if err != nil {
				return "", err
			}
		}
		return
	}}
}
