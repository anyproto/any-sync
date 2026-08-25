package iroh

import (
	"bytes"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/go-iroh/endpointticket"
	"github.com/tmc/go-iroh/relayserver"
)

// With a relay configured the ticket carries only the relay URL and a dial
// goes through the relay session (both sides register before use).
func TestIrohTransport_Relay(t *testing.T) {
	ts := httptest.NewServer(relayserver.New())
	defer ts.Close()
	conf := Config{RelayURLs: []string{ts.URL}}

	fxS := newFixtureConf(t, conf)
	defer fxS.finish(t)
	fxC := newFixtureConf(t, conf)
	defer fxC.finish(t)

	addr, err := endpointticket.Decode(fxS.Ticket())
	require.NoError(t, err)
	assert.Equal(t, fxS.ep.ID(), addr.ID)
	require.Len(t, addr.RelayURLs(), 1)
	assert.Equal(t, ts.URL+"/", addr.RelayURLs()[0].String())
	assert.Empty(t, addr.IPAddrs(), "relay ticket must not carry direct addrs")

	mcC, mcS := fxC.connect(t, fxS)

	done := make(chan string, 1)
	go func() {
		conn, serr := mcS.Accept()
		if serr != nil {
			done <- serr.Error()
			return
		}
		buf := bytes.NewBuffer(nil)
		_, _ = io.Copy(buf, conn)
		done <- buf.String()
	}()
	conn, err := mcC.Open(ctx)
	require.NoError(t, err)
	_, err = conn.Write([]byte("via relay"))
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	select {
	case got := <-done:
		assert.Equal(t, "via relay", got)
	case <-time.After(5 * time.Second):
		t.Fatal("no data through relay")
	}
}
