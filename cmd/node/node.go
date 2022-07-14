package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/config"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/service/account"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var log = logger.NewNamed("main")

var (
	flagConfigFile  = flag.String("c", "etc/config.yml", "path to config file")
	flagAccountFile = flag.String("a", "etc/account.yaml", "path to account file")
	flagVersion     = flag.Bool("v", false, "show version and exit")
	flagHelp        = flag.Bool("h", false, "show help and exit")
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

	// create app
	ctx := context.Background()
	a := new(app.App)

	// open config file
	conf, err := config.NewFromFile(*flagConfigFile)
	if err != nil {
		log.Fatal("can't open config file", zap.Error(err))
	}

	// open account file with node's keys
	acc, err := account.NewFromFile(*flagAccountFile)
	if err != nil {
		log.Fatal("can't open account file", zap.Error(err))
	}

	// bootstrap components
	a.Register(conf)
	a.Register(acc)
	Bootstrap(a)

	// start app
	if err := a.Start(ctx); err != nil {
		log.Error("can't start app", zap.Error(err))
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
}

func Bootstrap(a *app.App) {
	//a.Register(mycomponent.New())
}
