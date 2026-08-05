package peerservice

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLocalAddrDetector_IPLiterals(t *testing.T) {
	d := newLocalAddrDetector()
	d.resolve = func(_ context.Context, host string) ([]net.IPAddr, error) {
		t.Fatalf("unexpected resolve of %q for an ip literal", host)
		return nil, nil
	}

	for _, tc := range []struct {
		hostport string
		local    bool
	}{
		{"127.0.0.1:1111", true},
		{"[::1]:4242", true},
		{"::1", true},
		{"192.168.1.5:4242", true},
		{"10.0.0.7:1", true},
		{"172.18.0.5:1001", true},
		{"[::ffff:192.168.1.5]:4242", true},
		{"[fd00::1]:4242", true},
		{"169.254.1.2:1", true},
		{"[fe80::1%en0]:4242", true},
		{"203.0.113.1:443", false},
		{"8.8.8.8:53", false},
		{"100.64.1.2:4242", false}, // CGNAT (tailscale) — deliberately non-local
		{"0.0.0.0:1", false},
	} {
		assert.Equal(t, tc.local, d.isLocal(ctx, tc.hostport), tc.hostport)
	}
}

func TestLocalAddrDetector_Hostnames(t *testing.T) {
	t.Run("verdict expires after ttl", func(t *testing.T) {
		d := newLocalAddrDetector()
		now := time.Now()
		d.now = func() time.Time { return now }
		var calls int
		d.resolve = func(_ context.Context, _ string) ([]net.IPAddr, error) {
			calls++
			return []net.IPAddr{{IP: net.ParseIP("192.168.1.10")}}, nil
		}

		assert.True(t, d.isLocal(ctx, "box.lan:1111"))
		assert.True(t, d.isLocal(ctx, "box.lan:1111"))
		assert.Equal(t, 1, calls)

		now = now.Add(localResolveTTL + time.Second)
		assert.True(t, d.isLocal(ctx, "box.lan:1111"))
		assert.Equal(t, 2, calls)
	})

	t.Run("failed lookup gets short ttl", func(t *testing.T) {
		d := newLocalAddrDetector()
		now := time.Now()
		d.now = func() time.Time { return now }
		var calls int
		d.resolve = func(_ context.Context, _ string) ([]net.IPAddr, error) {
			calls++
			if calls == 1 {
				return nil, fmt.Errorf("dns unavailable")
			}
			return []net.IPAddr{{IP: net.ParseIP("192.168.1.10")}}, nil
		}

		assert.False(t, d.isLocal(ctx, "box.lan:1111"))
		now = now.Add(localResolveErrTTL + time.Second)
		assert.True(t, d.isLocal(ctx, "box.lan:1111"))
		assert.Equal(t, 2, calls)
	})

	t.Run("caller-canceled lookup is not cached", func(t *testing.T) {
		d := newLocalAddrDetector()
		d.resolve = func(rctx context.Context, _ string) ([]net.IPAddr, error) {
			return nil, rctx.Err()
		}
		canceled, cancel := context.WithCancel(context.Background())
		cancel()

		assert.False(t, d.isLocal(canceled, "box.lan:1111"))

		d.resolve = func(_ context.Context, _ string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("192.168.1.10")}}, nil
		}
		assert.True(t, d.isLocal(ctx, "box.lan:1111"))
	})

	t.Run("public hostname is not local", func(t *testing.T) {
		d := newLocalAddrDetector()
		d.resolve = func(_ context.Context, _ string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("142.250.185.78")}}, nil
		}
		assert.False(t, d.isLocal(ctx, "example.org:443"))
	})
}
