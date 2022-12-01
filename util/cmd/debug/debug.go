package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/client"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/stdin"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var log = logger.NewNamed("main")

var (
	// we can't use "v" here because of glog init (through badger) setting flag.Bool with "v"
	flagVersion = flag.Bool("ver", false, "show version and exit")
	flagHelp    = flag.Bool("h", false, "show help and exit")
)

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Println(app.VersionDescription())
		return
	}
	if *flagHelp {
		flag.PrintDefaults()
		return
	}

	if debug, ok := os.LookupEnv("ANYPROF"); ok && debug != "" {
		go func() {
			http.ListenAndServe(debug, nil)
		}()
	}

	// create app
	ctx := context.Background()
	a := new(app.App)
	Bootstrap(a)
	// start app
	if err := a.Start(ctx); err != nil {
		log.Fatal("can't start app", zap.Error(err))
	}
	log.Info("app started", zap.String("version", a.Version()))

	// wait exit signal
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	sig := <-exit
	log.Info("received exit signal, stop app...", zap.String("signal", fmt.Sprint(sig)))

	// close app
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := a.Close(ctx); err != nil {
		log.Fatal("close error", zap.Error(err))
	} else {
		log.Info("goodbye!")
	}
	time.Sleep(time.Second / 3)
}

func Bootstrap(a *app.App) {
	a.Register(client.New()).
		Register(api.New()).
		Register(stdin.New())
}
