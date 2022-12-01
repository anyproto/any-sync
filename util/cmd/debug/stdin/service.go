package stdin

import (
	"bufio"
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/peers"
	"os"
	"strings"
)

const CName = "debug.stdin"

var log = logger.NewNamed(CName).Sugar()

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
	for {
		str, err := reader.ReadString('\n')
		if err != nil {
			log.Errorf("Error in read string: %s", err)
			return
		}
		// trimming newline
		str = str[:len(str)-1]

		log.Debug(str)
		split := strings.Split(str, " ")
		if len(split) < 2 {
			log.Error("incorrect number of arguments")
			continue
		}

		peer, err := s.peers.Get(split[0])
		if err != nil {
			log.Error("no such peer")
			continue
		}
		res, err := s.api.Call(peer, split[1], split[2:])
		if err != nil {
			log.Errorf("Error in performing request: %s", err)
			return
		}
		log.Debug(res)
	}
}
