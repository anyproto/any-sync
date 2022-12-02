package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
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
	client := s.client
	s.clientCommands["create-space"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 0 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := client.CreateSpace(context.Background(), server.Address, &apiproto.CreateSpaceRequest{})
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
		resp, err := client.DeriveSpace(context.Background(), server.Address, &apiproto.DeriveSpaceRequest{})
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
		resp, err := client.CreateDocument(context.Background(), server.Address, &apiproto.CreateDocumentRequest{
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
		_, err = client.DeleteDocument(context.Background(), server.Address, &apiproto.DeleteDocumentRequest{
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
		if len(params) != 3 {
			err = ErrIncorrectParamsCount
			return
		}
		resp, err := client.AddText(context.Background(), server.Address, &apiproto.AddTextRequest{
			SpaceId:    params[0],
			DocumentId: params[1],
			Text:       params[2],
		})
		if err != nil {
			return
		}
		res = resp.DocumentId
		return
	}}
	s.clientCommands["load-space"] = Command{Cmd: func(server peers.Peer, params []string) (res string, err error) {
		if len(params) != 1 {
			err = ErrIncorrectParamsCount
			return
		}
		_, err = client.LoadSpace(context.Background(), server.Address, &apiproto.LoadSpaceRequest{
			SpaceId: params[0],
		})
		if err != nil {
			return
		}
		res = params[0]
		return
	}}
}

func (s *service) registerNodeCommands() {
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
			_, err := s.client.AddText(context.Background(), peer.Address, &apiproto.AddTextRequest{
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
