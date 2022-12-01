package stdin

import (
	"bufio"
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/client/api/apiproto"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api"
	"os"
)

const CName = "debug.stdin"

var log = logger.NewNamed(CName).Sugar()

type Service interface {
	app.ComponentRunnable
}

type service struct {
	api api.Service
}

func New() Service {
	return &service{}
}

func (s *service) Init(a *app.App) (err error) {
	s.api = a.MustComponent(api.CName).(api.Service)
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
	// create new reader from stdin
	reader := bufio.NewReader(os.Stdin)
	// start infinite loop to continuously listen to input
	for {
		// read by one line (enter pressed)
		str, err := reader.ReadString('\n')
		// check for errors
		if err != nil {
			// close channel just to inform others
			log.Errorf("Error in read string: %s", err)
			return
		}
		log.Debug(str)

		res, err := s.api.CreateSpace(context.Background(), "127.0.0.1:8090", &apiproto.CreateSpaceRequest{})
		if err != nil {
			log.Errorf("Error in performing request: %s", err)
			return
		}
		log.Debug(res)
	}
}
