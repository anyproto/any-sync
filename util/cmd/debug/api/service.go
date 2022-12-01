package api

import (
	"context"
	"errors"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api/client"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api/node"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/peers"
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

type Service interface {
	app.Component
	Call(server peers.Peer, cmdName string, params []string) (string, error)
}

type service struct {
	clientCommands map[string]Command
	nodeCommands   map[string]Command
}

func New() Service {
	return &service{clientCommands: map[string]Command{}, nodeCommands: map[string]Command{}}
}

func (s *service) Init(a *app.App) (err error) {
	s.registerClientCommands(a.MustComponent(client.CName).(client.Service))
	s.registerNodeCommands(a.MustComponent(node.CName).(node.Service))

	return nil
}

func (s *service) Name() (name string) {
	return CName
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

func (s *service) registerClientCommands(client client.Service) {
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
}

func (s *service) registerNodeCommands(node node.Service) {
}
