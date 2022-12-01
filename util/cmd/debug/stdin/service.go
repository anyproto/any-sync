package stdin

import (
	"bufio"
	"context"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/peers"
	"os"
	"strings"
)

const CName = "debug.stdin"

var log = logger.NewNamed(CName)

type Service interface {
	app.ComponentRunnable
}

type service struct {
	api   api.Service
	peers peers.Service
}

func New() Service {
	return &service{}
}

func (s *service) Init(a *app.App) (err error) {
	s.api = a.MustComponent(api.CName).(api.Service)
	s.peers = a.MustComponent(peers.CName).(peers.Service)
	return
}

func (s *service) Name() (name string) {
	return CName
}

func (s *service) Run(ctx context.Context) (err error) {
	go s.readStdin()
	return nil
}

func (s *service) Close(ctx context.Context) (err error) {
	return nil
}

func (s *service) readStdin() {
	reader := bufio.NewReader(os.Stdin)
Loop:
	for {
		fmt.Print("> ")
		str, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("error in read string:", err)
			return
		}
		// trimming newline
		str = str[:len(str)-1]

		split := strings.Split(str, " ")
		if len(split) < 2 {
			fmt.Println("incorrect number of arguments")
			continue
		}
		switch split[0] {
		case "script":
			res, err := s.api.Script(split[1], split[2:])
			if err != nil {
				fmt.Println("error in performing script:", err)
				continue Loop
			}
			fmt.Println(res)
			continue Loop
		case "cmd":
			break
		default:
			fmt.Println("incorrect input")
			continue Loop
		}

		peer, err := s.peers.Get(split[1])
		if err != nil {
			fmt.Println("no such peer", err)
			continue
		}

		res, err := s.api.Call(peer, split[2], split[3:])
		if err != nil {
			fmt.Println("error in performing command:", err)
			continue
		}
		fmt.Println(res)
	}
}
