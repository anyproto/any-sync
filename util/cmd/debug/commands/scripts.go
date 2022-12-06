package commands

import (
	"context"
	"fmt"
	clientproto "github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"math/rand"
	"strings"
	"sync"
)

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
				addr, ok := s.peers[cl]
				if !ok {
					fmt.Println("no such client")
					return
				}

				addresses = append(addresses, addr)
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
