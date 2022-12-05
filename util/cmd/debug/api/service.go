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

	clientCmd := &cobra.Command{Use: "client commands"}
	clientCmd.PersistentFlags().StringP("client", "c", "", "the alias of the client")
	clientCmd.MarkFlagRequired("client")
	for _, cmd := range s.clientCommands {
		clientCmd.AddCommand(cmd)
	}
	rootCmd.AddCommand(clientCmd)

	nodeCmd := &cobra.Command{Use: "node commands"}
	nodeCmd.PersistentFlags().StringP("node", "n", "", "the alias of the node")
	nodeCmd.MarkFlagRequired("node")
	for _, cmd := range s.nodeCommands {
		nodeCmd.AddCommand(cmd)
	}
	rootCmd.AddCommand(nodeCmd)

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
	//s.registerScripts()

	return nil
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) registerClientCommands() {
	cmdCreateSpace := &cobra.Command{
		Use:   "create-space [params]",
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
		Use:   "load-space [params]",
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
		Use:   "derive-space [params]",
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
		Use:   "create-document [params]",
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
		Use:   "delete-document [params]",
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
			fmt.Println("deleted", args[1])
		},
	}
	cmdDeleteDocument.Flags().String("space", "", "the space where something is happening :-)")
	cmdDeleteDocument.MarkFlagRequired("space")
	s.clientCommands = append(s.clientCommands, cmdDeleteDocument)

	cmdAddText := &cobra.Command{
		Use:   "add-text [params]",
		Short: "add text to the document in the particular space",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			space, _ := cmd.Flags().GetString("space")
			document, _ := cmd.Flags().GetString("document")
			server, err := s.peers.Get(cli)
			if err != nil {
				fmt.Println("no such client")
				return
			}
			resp, err := s.client.AddText(context.Background(), server.Address, &clientproto.AddTextRequest{
				SpaceId:    space,
				DocumentId: document,
				Text:       args[0],
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
	cmdAddText.MarkFlagRequired("space")
	cmdAddText.MarkFlagRequired("document")
	s.clientCommands = append(s.clientCommands, cmdAddText)

	cmdAllTrees := &cobra.Command{
		Use:   "all-trees [params]",
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
		Use:   "dump-tree [params]",
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
		Use:   "tree-params [params]",
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
		Use:   "all-spaces [params]",
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
		Use:   "all-trees [params]",
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
		Use:   "dump-tree [params]",
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
		Use:   "tree-params [params]",
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
		Use:   "all-spaces [params]",
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

//
//func (s *service) registerScripts() {
//	s.scripts["create-many"] = Script{Cmd: func(params []string) (res string, err error) {
//		if len(params) != 5 {
//			err = ErrIncorrectParamsCount
//			return
//		}
//		peer, err := s.peers.Get(params[0])
//		if err != nil {
//			return
//		}
//		last, err := strconv.Atoi(params[4])
//		if err != nil {
//			return
//		}
//		if last <= 0 {
//			err = fmt.Errorf("incorrect number of steps")
//			return
//		}
//
//		for i := 0; i < last; i++ {
//			_, err := s.client.AddText(context.Background(), peer.Address, &clientproto.AddTextRequest{
//				SpaceId:    params[1],
//				DocumentId: params[2],
//				Text:       params[3],
//			})
//			if err != nil {
//				return "", err
//			}
//		}
//		return
//	}}
//	s.scripts["create-many-two-clients"] = Script{Cmd: func(params []string) (res string, err error) {
//		if len(params) != 6 {
//			err = ErrIncorrectParamsCount
//			return
//		}
//		peer1, err := s.peers.Get(params[0])
//		if err != nil {
//			return
//		}
//		peer2, err := s.peers.Get(params[1])
//		if err != nil {
//			return
//		}
//		last, err := strconv.Atoi(params[5])
//		if err != nil {
//			return
//		}
//		if last <= 0 {
//			err = fmt.Errorf("incorrect number of steps")
//			return
//		}
//		wg := &sync.WaitGroup{}
//		var mError errs.Group
//		createMany := func(peer peers.Peer) {
//			defer wg.Done()
//			for i := 0; i < last; i++ {
//				_, err := s.client.AddText(context.Background(), peer.Address, &clientproto.AddTextRequest{
//					SpaceId:    params[2],
//					DocumentId: params[3],
//					Text:       params[4],
//					IsSnapshot: rand.Int()%2 == 0,
//				})
//				if err != nil {
//					mError.Add(err)
//					return
//				}
//			}
//		}
//		for _, p := range []peers.Peer{peer1, peer2} {
//			wg.Add(1)
//			createMany(p)
//		}
//		wg.Wait()
//		if mError.Err() != nil {
//			err = mError.Err()
//			return
//		}
//		return
//	}}
//}
