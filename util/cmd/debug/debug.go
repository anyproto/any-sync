package main

import (
	"context"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/common/app/logger"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api/client"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/api/node"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/drpcclient"
	"github.com/anytypeio/go-anytype-infrastructure-experiments/util/cmd/debug/peers"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
	"os"
)

var log = logger.NewNamed("main")

func init() {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder

	logFile, err := os.OpenFile("debug.json", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	core := zapcore.NewCore(zapcore.NewJSONEncoder(config), zapcore.AddSync(logFile), zapcore.DebugLevel)
	logger.SetDefault(zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)))
}

func main() {
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
}

func Bootstrap(a *app.App) {
	a.Register(drpcclient.New()).
		Register(peers.New()).
		Register(client.New()).
		Register(node.New()).
		Register(api.New())
}
