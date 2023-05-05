package metric

import (
	"context"
	"github.com/anytypeio/any-sync/app/logger"
	"testing"
)

func TestLog(t *testing.T) {
	m := &metric{rpcLog: logger.NewNamed("rpcLog")}
	m.RequestLog(context.Background(), "rpc")
}
