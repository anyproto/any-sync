package commands

import (
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/commands/client"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/commands/node"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

const CName = "debug.commands"

var log = logger.NewNamed(CName)

type Service interface {
	app.ComponentRunnable
}

type service struct {
	client         client.Service
	node           node.Service
	peers          map[string]string
	clientCommands []*cobra.Command
	nodeCommands   []*cobra.Command
	scripts        []*cobra.Command
}

func New() Service {
	return &service{}
}

func (s *service) Close(ctx context.Context) (err error) {
	return nil
}

func (s *service) Init(a *app.App) (err error) {
	s.client = a.MustComponent(client.CName).(client.Service)
	s.node = a.MustComponent(node.CName).(node.Service)
	s.peers = map[string]string{}
	s.registerClientCommands()
	s.registerNodeCommands()
	s.registerScripts()

	return nil
}

func (s *service) Run(ctx context.Context) (err error) {
	rootCmd := &cobra.Command{Use: "debug", PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cfgPath, err := cmd.Flags().GetString("config")
		if err != nil {
			panic(fmt.Sprintf("no config flag is registered: %s", err.Error()))
		}
		err = s.parseAddresses(cfgPath)
		if err != nil {
			panic(fmt.Sprintf("couldn't load config with addresses of nodes: %s", err.Error()))
		}
	}}
	rootCmd.PersistentFlags().String("config", "util/cmd/nodesgen/nodemap.yml", "nodes configuration")

	clientCmd := &cobra.Command{Use: "client commands to be executed on a specified client"}
	clientCmd.PersistentFlags().StringP("client", "c", "", "the alias of the client")
	clientCmd.MarkFlagRequired("client")
	for _, cmd := range s.clientCommands {
		clientCmd.AddCommand(cmd)
	}
	rootCmd.AddCommand(clientCmd)

	nodeCmd := &cobra.Command{Use: "node commands to be executed on a node"}
	nodeCmd.PersistentFlags().StringP("node", "n", "", "the alias of the node")
	nodeCmd.MarkFlagRequired("node")
	for _, cmd := range s.nodeCommands {
		nodeCmd.AddCommand(cmd)
	}
	rootCmd.AddCommand(nodeCmd)

	scriptsCmd := &cobra.Command{Use: "script which can have arbitrary params and can include mutliple clients and nodes"}
	for _, cmd := range s.scripts {
		scriptsCmd.AddCommand(cmd)
	}
	rootCmd.AddCommand(scriptsCmd)

	return rootCmd.Execute()
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) parseAddresses(path string) (err error) {
	nodesMap := &NodesMap{}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, nodesMap)
	if err != nil {
		return err
	}

	for idx, n := range nodesMap.Nodes {
		s.peers[fmt.Sprintf("node%d", idx+1)] = n.APIAddresses[0]
	}
	for idx, c := range nodesMap.Clients {
		s.peers[fmt.Sprintf("client%d", idx+1)] = c.APIAddresses[0]
	}
	return nil
}
