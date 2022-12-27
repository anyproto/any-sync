package commands

import (
	"context"
	"fmt"
	clientproto "github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	nodeproto "github.com/anytypeio/go-anytype-infrastructure-experiments/node/api/apiproto"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"golang.org/x/exp/slices"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type DebugClient struct {
	name    string
	address string
}

func keepChecking(fn func() bool, finished chan bool) {
	ticker := time.NewTicker(time.Second * 1)

	go func() {
		for {
			select {
			case <-ticker.C:
				if fn() {
					finished <- true
					ticker.Stop()
					return
				}
			}
		}
	}()

	<-finished
}

func containsFunc[E comparable](s []E, f func(E) bool) bool {
	return slices.IndexFunc(s, f) >= 0
}

func iterate(shouldStop func(iteration int) bool, wg *sync.WaitGroup, iterationCount int) {
	ticker := time.NewTicker(time.Millisecond * 10)
	defer wg.Done()
	defer ticker.Stop()
	for i := 0; i < iterationCount; i++ {
		select {
		case <-ticker.C:
			if shouldStop(i) {
				return
			}
		}
	}
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
						IsSnapshot: rand.Int()%10 == 0,
					})
					if err != nil {
						mError.Add(err)
						return
					}
				}
			}
			for _, p := range addresses {
				wg.Add(1)
				go createMany(p)
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

	integration := &cobra.Command{
		Use:   "integration-tests",
		Short: "integration-tests",
		Args:  cobra.RangeArgs(0, 0),
		Run: func(cmd *cobra.Command, args []string) {
			documentsCount, _ := cmd.Flags().GetInt("documents")

			node1, node2, node3 :=
				DebugClient{
					name:    "Node1",
					address: s.peers["node1"],
				},
				DebugClient{
					name:    "Node2",
					address: s.peers["node2"],
				},
				DebugClient{
					name:    "Node3",
					address: s.peers["node3"],
				}

			nodes := []DebugClient{node1, node2, node3}

			start := time.Now()
			elapsedFunc := func() time.Duration { return time.Since(start) }

			print := func(output string) {
				fmt.Printf("%s | %s \n", output, elapsedFunc())
			}

			createSpace := func(client DebugClient) string {
				space, err := s.client.CreateSpace(context.Background(), client.address, &clientproto.CreateSpaceRequest{})

				if err != nil {
					panic("can't create a space")
				}

				print(fmt.Sprintf("%s: Created a space with id %s", client.name, space.Id))

				return space.Id
			}

			nodesDocumentHeads := func(docId string, spaceId string) map[string][]string {
				var dictionary = map[string][]string{}
				for _, node := range nodes {
					resp, _ := s.node.TreeParams(context.Background(), node.address, &nodeproto.TreeParamsRequest{
						SpaceId:    spaceId,
						DocumentId: docId,
					})

					dictionary[node.name] = resp.GetHeadIds()
				}

				return dictionary
			}

			waitUntilLoadSpace := func(spaceId string, client DebugClient) bool {
				print(fmt.Sprintf("%s: Trying to load space with id %s", client.name, spaceId))

				_, err := s.client.LoadSpace(context.Background(), client.address, &clientproto.LoadSpaceRequest{spaceId})

				if err != nil {
					return false
				}

				print(fmt.Sprintf("%s: Did load space with id %s", client.name, spaceId))

				return true
			}

			waitUntilSpaceExists := func(groupId string, client DebugClient) bool {
				allSpaces, err := s.client.AllSpaces(context.Background(), client.address, &clientproto.AllSpacesRequest{})
				if err != nil {
					panic("can't retrieve all spaces")
				}

				print(fmt.Sprintf("%s: contains %s, %t", client, groupId, slices.Contains(allSpaces.SpaceIds, groupId)))
				return slices.Contains(allSpaces.SpaceIds, groupId)
			}

			waitUntilDocumentExists := func(client DebugClient, spaceId string, documentId string) bool {
				rs, _ := s.client.AllTrees(context.Background(), client.address, &clientproto.AllTreesRequest{SpaceId: spaceId})

				contains := containsFunc(rs.Trees, func(c *clientproto.Tree) bool {
					return c.Id == documentId
				})

				if !contains {
					print(fmt.Sprintf("%s doesn't contain a document %s", client.name, documentId))
					fmt.Println(nodesDocumentHeads(documentId, spaceId))
				} else {
					print(fmt.Sprintf("%s contains a document %s", client.name, documentId))
				}

				return contains
			}

			addTextFunc := func(iterationNumber int, client DebugClient, spaceId string, documentId string) bool {
				randText := func(nonce int) string {
					rand.Seed(time.Now().UnixNano())
					buf := make([]byte, nonce*3)
					rand.Read(buf)
					return string(buf)
				}

				r, err := s.client.AddText(context.Background(), client.address, &clientproto.AddTextRequest{
					SpaceId:    spaceId,
					DocumentId: documentId,
					Text:       randText(iterationNumber),
					IsSnapshot: false,
				})

				if err != nil {
					print(client.address + err.Error())
					return true
				} else {
					print(fmt.Sprintf("%s: Did add text to document %s, head: %s, text: %d", client.name, r.DocumentId, r.HeadId, iterationNumber))
				}

				return false
			}

			createDocumentFunc := func(client DebugClient, spaceId string) string {
				docResponse, err := s.client.CreateDocument(context.Background(), client.address, &clientproto.CreateDocumentRequest{SpaceId: spaceId})

				if err != nil {
					panic("can't create a document")
				}

				print(fmt.Sprintf("%s: Created a document in space %s with id %s", client.name, spaceId, docResponse.Id))
				return docResponse.Id
			}

			_ = func(wg *sync.WaitGroup, client DebugClient, spaceId string, docId string) {
				time.Sleep(2)

				documentDeletion := func() bool {
					print("Will remove document with id" + spaceId + "." + docId)

					_, err := s.client.DeleteDocument(context.Background(), client.address, &clientproto.DeleteDocumentRequest{
						SpaceId:    spaceId,
						DocumentId: docId,
					})

					return err == nil
				}

				var finita = make(chan bool)
				go keepChecking(func() bool { return documentDeletion() }, finita)

				wg.Done()
			}

			checkHeadsEqual := func(client1 DebugClient, client2 DebugClient, spaceId string, docId string) bool {
				rs1, _ := s.client.TreeParams(context.Background(), client1.address, &clientproto.TreeParamsRequest{
					SpaceId:    spaceId,
					DocumentId: docId,
				})
				rs2, _ := s.client.TreeParams(context.Background(), client2.address, &clientproto.TreeParamsRequest{
					SpaceId:    spaceId,
					DocumentId: docId,
				})

				print(fmt.Sprintf("%s document %s head is %v", client1.name, docId, rs1.GetHeadIds()))
				print(fmt.Sprintf("%s document %s head is %v", client2.name, docId, rs2.GetHeadIds()))

				if rs1.GetHeadIds() == nil || rs1.GetHeadIds() == nil {
					fmt.Println(nodesDocumentHeads(docId, spaceId))
					print("Some head is nil")
					return false
				}

				return slices.Equal(rs1.GetHeadIds(), rs2.GetHeadIds())
			}

			client1, client2 := DebugClient{
				name:    "Client1",
				address: s.peers["client1"],
			}, DebugClient{
				name:    "Client2",
				address: s.peers["client2"],
			}

			singleTest := func(testGroup *sync.WaitGroup, spaceId string) {
				wg := sync.WaitGroup{}
				finished := make(chan bool)

				documentId := createDocumentFunc(client2, spaceId)
				keepChecking(func() bool { return waitUntilDocumentExists(client1, spaceId, documentId) }, finished)

				wg.Add(2)
				go iterate(func(iteration int) bool {
					return addTextFunc(iteration, client2, spaceId, documentId)
				}, &wg, 70)
				go iterate(func(iteration int) bool {
					return addTextFunc(iteration, client1, spaceId, documentId)
				}, &wg, 100)

				wg.Wait()

				keepChecking(func() bool { return checkHeadsEqual(client1, client2, spaceId, documentId) }, finished)

				print("The End")

				testGroup.Done()
			}

			// Start
			finished := make(chan bool)
			spaceId := createSpace(client1)

			keepChecking(func() bool { return waitUntilLoadSpace(spaceId, client2) }, finished)
			keepChecking(func() bool { return waitUntilSpaceExists(spaceId, client2) }, finished)

			wg := sync.WaitGroup{}

			wg.Add(documentsCount)
			for i := 0; i < documentsCount; i++ {
				go singleTest(&wg, spaceId)
			}

			wg.Wait()

			print("Directed by robert b weide")
		},
	}

	integration.Flags().Int("documents", 50, "how many documents would be created")
	s.scripts = append(s.scripts, integration)
}
