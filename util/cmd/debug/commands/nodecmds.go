package commands

import (
	"context"
	"fmt"
	nodeproto "github.com/anytypeio/go-anytype-infrastructure-experiments/node/debug/nodedebugrpc/nodedebugrpcproto"
	"github.com/spf13/cobra"
)

func (s *service) registerNodeCommands() {
	cmdAllTrees := &cobra.Command{
		Use:   "all-trees [space]",
		Short: "print all trees in space and their heads",
		Args:  cobra.RangeArgs(1, 1),
		Run: func(cmd *cobra.Command, args []string) {
			nd, _ := cmd.Flags().GetString("node")
			addr, ok := s.peers[nd]
			if !ok {
				fmt.Println("no such node")
				return
			}

			resp, err := s.node.AllTrees(context.Background(), addr, &nodeproto.AllTreesRequest{
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
			addr, ok := s.peers[nd]
			if !ok {
				fmt.Println("no such node")
				return
			}

			resp, err := s.node.DumpTree(context.Background(), addr, &nodeproto.DumpTreeRequest{
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
			addr, ok := s.peers[nd]
			if !ok {
				fmt.Println("no such node")
				return
			}

			resp, err := s.node.TreeParams(context.Background(), addr, &nodeproto.TreeParamsRequest{
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
			addr, ok := s.peers[nd]
			if !ok {
				fmt.Println("no such node")
				return
			}

			resp, err := s.node.AllSpaces(context.Background(), addr, &nodeproto.AllSpacesRequest{})
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
