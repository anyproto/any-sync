package commands

import (
	"context"
	"fmt"
	clientproto "github.com/anytypeio/go-anytype-infrastructure-experiments/client/debug/clientdebugrpc/clientdebugrpcproto"
	"github.com/spf13/cobra"
)

func (s *service) registerClientCommands() {
	cmdCreateSpace := &cobra.Command{
		Use:   "create-space",
		Short: "create the space",
		Args:  cobra.RangeArgs(0, 0),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}
			resp, err := s.client.CreateSpace(context.Background(), addr, &clientproto.CreateSpaceRequest{})
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
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}

			_, err := s.client.LoadSpace(context.Background(), addr, &clientproto.LoadSpaceRequest{
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
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}

			resp, err := s.client.DeriveSpace(context.Background(), addr, &clientproto.DeriveSpaceRequest{})
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
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}
			resp, err := s.client.CreateDocument(context.Background(), addr, &clientproto.CreateDocumentRequest{
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
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}
			_, err := s.client.DeleteDocument(context.Background(), addr, &clientproto.DeleteDocumentRequest{
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
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}

			resp, err := s.client.AddText(context.Background(), addr, &clientproto.AddTextRequest{
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
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}

			resp, err := s.client.AllTrees(context.Background(), addr, &clientproto.AllTreesRequest{
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
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}

			resp, err := s.client.DumpTree(context.Background(), addr, &clientproto.DumpTreeRequest{
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
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}

			resp, err := s.client.TreeParams(context.Background(), addr, &clientproto.TreeParamsRequest{
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
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}

			resp, err := s.client.AllSpaces(context.Background(), addr, &clientproto.AllSpacesRequest{})
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

	cmdTreeWatch := &cobra.Command{
		Use:   "tree-watch [document]",
		Short: "start watching the tree (prints in logs the status on the client side)",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			space, _ := cmd.Flags().GetString("space")
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}

			_, err := s.client.Watch(context.Background(), addr, &clientproto.WatchRequest{
				SpaceId: space,
				TreeId:  args[0],
			})
			if err != nil {
				fmt.Println("couldn't start watching tree", err)
				return
			}
			fmt.Println(args[0])
		},
	}
	cmdTreeWatch.Flags().String("space", "", "the space where something is happening :-)")
	cmdTreeWatch.MarkFlagRequired("space")
	s.clientCommands = append(s.clientCommands, cmdTreeWatch)

	cmdTreeUnwatch := &cobra.Command{
		Use:   "tree-unwatch [document]",
		Short: "stop watching the tree (prints in logs the status on the client side)",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := cmd.Flags().GetString("client")
			space, _ := cmd.Flags().GetString("space")
			addr, ok := s.peers[cli]
			if !ok {
				fmt.Println("no such client")
				return
			}

			_, err := s.client.Unwatch(context.Background(), addr, &clientproto.UnwatchRequest{
				SpaceId: space,
				TreeId:  args[0],
			})
			if err != nil {
				fmt.Println("couldn't stop watching tree", err)
				return
			}
			fmt.Println(args[0])
		},
	}
	cmdTreeUnwatch.Flags().String("space", "", "the space where something is happening :-)")
	cmdTreeUnwatch.MarkFlagRequired("space")
	s.clientCommands = append(s.clientCommands, cmdTreeUnwatch)
}
