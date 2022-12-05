package api

import (
	"context"
	"fmt"
	clientproto "github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	nodeproto "github.com/anytypeio/go-anytype-infrastructure-experiments/node/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api/client"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api/node"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/peers"
	"github.com/spf13/cobra"
	_ "github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"math/rand"
	"strings"
	"sync"
)

const CName = "debug.api"

var log = logger.NewNamed(CName)

type Service interface {
	app.ComponentRunnable
}

type service struct {
	client         client.Service
	node           node.Service
	peers          peers.Service
	clientCommands []*cobra.Command
	nodeCommands   []*cobra.Command
	scripts        []*cobra.Command
}

func New() Service {
	return &service{}
}

func (s *service) Run(ctx context.Context) (err error) {
	rootCmd := &cobra.Command{Use: "debug"}

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

func (s *service) Close(ctx context.Context) (err error) {
	return nil
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

func (s *service) registerClientCommands() {
	cmdCreateSpace := &cobra.Command{
		Use:   "create-space",
		Short: "create the space",
		Args:  cobra.RangeArgs(0, 0),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			server, err := s.peers.Get(cli)
			if err != nil {
				fmt.Println("no such client")
				return
			}
			resp, err := s.client.CreateSpace(context.Background(), server.Address, &clientproto.CreateSpaceRequest{})
			if err != nil {
				fmt.Println("couldn't create a space", err)
				return
			}
			fmt.Println(resp.Id)
		},
	}
	s.clientCommands = append(s.clientCommands, cmdCreateSpace)

	cmdLoadSpace := &cobra.Command{
		Use:   "load-space [space]",
		Short: "load the space",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			server, err := s.peers.Get(cli)
			if err != nil {
				fmt.Println("no such client")
				return
			}
			_, err = s.client.LoadSpace(context.Background(), server.Address, &clientproto.LoadSpaceRequest{
				SpaceId: args[0],
			})
			if err != nil {
				fmt.Println("couldn't load the space", err)
				return
			}
			fmt.Println("space loaded", args[0])
		},
	}
	s.clientCommands = append(s.clientCommands, cmdLoadSpace)

	cmdDeriveSpace := &cobra.Command{
		Use:   "derive-space",
		Short: "derive the space from account data",
		Args:  cobra.RangeArgs(0, 0),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			server, err := s.peers.Get(cli)
			if err != nil {
				fmt.Println("no such client")
				return
			}
			resp, err := s.client.DeriveSpace(context.Background(), server.Address, &clientproto.DeriveSpaceRequest{})
			if err != nil {
				fmt.Println("couldn't derive a space", err)
				return
			}
			fmt.Println(resp.Id)
		},
	}
	s.clientCommands = append(s.clientCommands, cmdDeriveSpace)

	cmdCreateDocument := &cobra.Command{
		Use:   "create-document [space]",
		Short: "create the document in a particular space",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			server, err := s.peers.Get(cli)
			if err != nil {
				fmt.Println("no such client")
				return
			}
			resp, err := s.client.CreateDocument(context.Background(), server.Address, &clientproto.CreateDocumentRequest{
				SpaceId: args[0],
			})
			if err != nil {
				fmt.Println("couldn't create a document", err)
				return
			}
			fmt.Println(resp.Id)
		},
	}
	s.clientCommands = append(s.clientCommands, cmdCreateDocument)

	cmdDeleteDocument := &cobra.Command{
		Use:   "delete-document [document]",
		Short: "delete the document in a particular space",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			space, _ := cmd.Flags().GetString("space")
			server, err := s.peers.Get(cli)
			if err != nil {
				fmt.Println("no such client")
				return
			}
			_, err = s.client.DeleteDocument(context.Background(), server.Address, &clientproto.DeleteDocumentRequest{
				SpaceId:    space,
				DocumentId: args[0],
			})
			if err != nil {
				fmt.Println("couldn't delete the document", err)
				return
			}
			fmt.Println("deleted", args[0])
		},
	}
	cmdDeleteDocument.Flags().String("space", "", "the space where something is happening :-)")
	cmdDeleteDocument.MarkFlagRequired("space")
	s.clientCommands = append(s.clientCommands, cmdDeleteDocument)

	cmdAddText := &cobra.Command{
		Use:   "add-text [text]",
		Short: "add text to the document in the particular space",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			space, _ := cmd.Flags().GetString("space")
			document, _ := cmd.Flags().GetString("document")
			snapshot, _ := cmd.Flags().GetBool("snapshot")
			server, err := s.peers.Get(cli)
			if err != nil {
				fmt.Println("no such client")
				return
			}
			resp, err := s.client.AddText(context.Background(), server.Address, &clientproto.AddTextRequest{
				SpaceId:    space,
				DocumentId: document,
				Text:       args[0],
				IsSnapshot: snapshot,
			})
			if err != nil {
				fmt.Println("couldn't add text to the document", err)
				return
			}
			fmt.Println("added text", resp.DocumentId, "root:", resp.RootId, "head:", resp.HeadId)
		},
	}
	cmdAddText.Flags().String("space", "", "the space where something is happening :-)")
	cmdAddText.Flags().String("document", "", "the document where something is happening :-)")
	cmdAddText.Flags().Bool("snapshot", false, "tells if the snapshot should be created")
	cmdAddText.MarkFlagRequired("space")
	cmdAddText.MarkFlagRequired("document")
	s.clientCommands = append(s.clientCommands, cmdAddText)

	cmdAllTrees := &cobra.Command{
		Use:   "all-trees [space]",
		Short: "print all trees in space and their heads",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			server, err := s.peers.Get(cli)
			if err != nil {
				fmt.Println("no such client")
				return
			}
			resp, err := s.client.AllTrees(context.Background(), server.Address, &clientproto.AllTreesRequest{
				SpaceId: args[0],
			})
			if err != nil {
				fmt.Println("couldn't print all the trees", err)
				return
			}
			var res string
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
			fmt.Println(res)
		},
	}
	s.clientCommands = append(s.clientCommands, cmdAllTrees)

	cmdDumpTree := &cobra.Command{
		Use:   "dump-tree [document]",
		Short: "get graphviz description of the tree",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			space, _ := cmd.Flags().GetString("space")
			server, err := s.peers.Get(cli)
			if err != nil {
				fmt.Println("no such client")
				return
			}

			resp, err := s.client.DumpTree(context.Background(), server.Address, &clientproto.DumpTreeRequest{
				SpaceId:    space,
				DocumentId: args[0],
			})
			if err != nil {
				fmt.Println("couldn't dump the tree", err)
				return
			}
			fmt.Println(resp.Dump)
		},
	}
	cmdDumpTree.Flags().String("space", "", "the space where something is happening :-)")
	cmdDumpTree.MarkFlagRequired("space")
	s.clientCommands = append(s.clientCommands, cmdDumpTree)

	cmdTreeParams := &cobra.Command{
		Use:   "tree-params [document]",
		Short: "print heads and root of the tree",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			space, _ := cmd.Flags().GetString("space")
			server, err := s.peers.Get(cli)
			if err != nil {
				fmt.Println("no such client")
				return
			}

			resp, err := s.client.TreeParams(context.Background(), server.Address, &clientproto.TreeParamsRequest{
				SpaceId:    space,
				DocumentId: args[0],
			})
			if err != nil {
				fmt.Println("couldn't print params of the tree", err)
				return
			}
			res := resp.RootId + "->"
			for headIdx, head := range resp.HeadIds {
				res += head
				if headIdx != len(resp.HeadIds)-1 {
					res += ","
				}
			}
			fmt.Println(res)
		},
	}
	cmdTreeParams.Flags().String("space", "", "the space where something is happening :-)")
	cmdTreeParams.MarkFlagRequired("space")
	s.clientCommands = append(s.clientCommands, cmdTreeParams)

	cmdAllSpaces := &cobra.Command{
		Use:   "all-spaces",
		Short: "print all spaces",
		Args:  cobra.RangeArgs(0, 0),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			server, err := s.peers.Get(cli)
			if err != nil {
				fmt.Println("no such client")
				return
			}
			resp, err := s.client.AllSpaces(context.Background(), server.Address, &clientproto.AllSpacesRequest{})
			if err != nil {
				fmt.Println("couldn't print all the spaces", err)
				return
			}
			var res string
			for treeIdx, spaceId := range resp.SpaceIds {
				res += spaceId
				if treeIdx != len(resp.SpaceIds)-1 {
					res += "\n"
				}
			}
			fmt.Println(res)
		},
	}
	s.clientCommands = append(s.clientCommands, cmdAllSpaces)
}

func (s *service) registerNodeCommands() {
	cmdAllTrees := &cobra.Command{
		Use:   "all-trees [space]",
		Short: "print all trees in space and their heads",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			nd, _ := cmd.Flags().GetString("node")
			server, err := s.peers.Get(nd)
			if err != nil {
				fmt.Println("no such node")
				return
			}
			resp, err := s.node.AllTrees(context.Background(), server.Address, &nodeproto.AllTreesRequest{
				SpaceId: args[0],
			})
			if err != nil {
				fmt.Println("couldn't print all the trees", err)
				return
			}
			var res string
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
			fmt.Println(res)
		},
	}
	s.nodeCommands = append(s.nodeCommands, cmdAllTrees)

	cmdDumpTree := &cobra.Command{
		Use:   "dump-tree [space]",
		Short: "get graphviz description of the tree",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			nd, _ := cmd.Flags().GetString("node")
			space, _ := cmd.Flags().GetString("space")
			server, err := s.peers.Get(nd)
			if err != nil {
				fmt.Println("no such node")
				return
			}

			resp, err := s.node.DumpTree(context.Background(), server.Address, &nodeproto.DumpTreeRequest{
				SpaceId:    space,
				DocumentId: args[0],
			})
			if err != nil {
				fmt.Println("couldn't dump the tree", err)
				return
			}
			fmt.Println(resp.Dump)
		},
	}
	cmdDumpTree.Flags().String("space", "", "the space where something is happening :-)")
	cmdDumpTree.MarkFlagRequired("space")
	s.nodeCommands = append(s.nodeCommands, cmdDumpTree)

	cmdTreeParams := &cobra.Command{
		Use:   "tree-params [document]",
		Short: "print heads and root of the tree",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			nd, _ := cmd.Flags().GetString("node")
			space, _ := cmd.Flags().GetString("space")
			server, err := s.peers.Get(nd)
			if err != nil {
				fmt.Println("no such node")
				return
			}

			resp, err := s.node.TreeParams(context.Background(), server.Address, &nodeproto.TreeParamsRequest{
				SpaceId:    space,
				DocumentId: args[0],
			})
			if err != nil {
				fmt.Println("couldn't print params of the tree", err)
				return
			}
			res := resp.RootId + "->"
			for headIdx, head := range resp.HeadIds {
				res += head
				if headIdx != len(resp.HeadIds)-1 {
					res += ","
				}
			}
			fmt.Println(res)
		},
	}
	cmdTreeParams.Flags().String("space", "", "the space where something is happening :-)")
	cmdTreeParams.MarkFlagRequired("space")
	s.nodeCommands = append(s.nodeCommands, cmdTreeParams)

	cmdAllSpaces := &cobra.Command{
		Use:   "all-spaces",
		Short: "print all spaces",
		Args:  cobra.RangeArgs(0, 0),
		Run: func(cmd *cobra.Command, args []string) {
			nd, _ := cmd.Flags().GetString("node")
			server, err := s.peers.Get(nd)
			if err != nil {
				fmt.Println("no such node")
				return
			}
			resp, err := s.node.AllSpaces(context.Background(), server.Address, &nodeproto.AllSpacesRequest{})
			if err != nil {
				fmt.Println("couldn't print all the spaces", err)
				return
			}
			var res string
			for treeIdx, spaceId := range resp.SpaceIds {
				res += spaceId
				if treeIdx != len(resp.SpaceIds)-1 {
					res += "\n"
				}
			}
			fmt.Println(res)
		},
	}
	s.nodeCommands = append(s.nodeCommands, cmdAllSpaces)
}

func (s *service) registerScripts() {
	cmdAddTextMany := &cobra.Command{
		Use:   "add-text-many [text]",
		Short: "add text to the document in the particular space in many clients at the same time with randomized snapshots",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			clients, _ := cmd.Flags().GetString("clients")
			space, _ := cmd.Flags().GetString("space")
			document, _ := cmd.Flags().GetString("document")
			times, _ := cmd.Flags().GetInt("times")
			if times <= 0 {
				fmt.Println("the times parameter should be more than 0")
				return
			}
			var addresses []string
			for _, cl := range strings.Split(clients, ",") {
				if len(cl) == 0 {
					continue
				}
				server, err := s.peers.Get(cl)
				if err != nil {
					fmt.Println("no such client")
					return
				}
				addresses = append(addresses, server.Address)
			}

			wg := &sync.WaitGroup{}
			var mError errs.Group
			createMany := func(address string) {
				defer wg.Done()
				for i := 0; i < times; i++ {
					_, err := s.client.AddText(context.Background(), address, &clientproto.AddTextRequest{
						SpaceId:    space,
						DocumentId: document,
						Text:       args[0],
						IsSnapshot: rand.Int()%2 == 0,
					})
					if err != nil {
						mError.Add(err)
						return
					}
				}
			}
			for _, p := range addresses {
				wg.Add(1)
				createMany(p)
			}
			wg.Wait()
			if mError.Err() != nil {
				fmt.Println("got errors while executing add many", mError.Err())
				return
			}
			return
		},
	}
	cmdAddTextMany.Flags().String("space", "", "the space where something is happening :-)")
	cmdAddTextMany.Flags().String("document", "", "the document where something is happening :-)")
	cmdAddTextMany.Flags().String("clients", "", "the aliases of clients with value separated by comma")
	cmdAddTextMany.Flags().Int("times", 1, "how many times we should add the change")
	cmdAddTextMany.MarkFlagRequired("space")
	cmdAddTextMany.MarkFlagRequired("document")
	cmdAddTextMany.MarkFlagRequired("clients")
	s.scripts = append(s.scripts, cmdAddTextMany)
}
