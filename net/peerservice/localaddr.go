package peerservice

import (
	"context"
	"net"
	"sync"
	"time"
)

const (
	// localResolveTimeout bounds the hostname lookup used only to order dial
	// candidates. Locally answered names (hosts file, docker embedded DNS,
	// mDNS cache, LAN resolver) reply within a few milliseconds; a lookup
	// that needs upstream recursion misses the budget and the address is
	// treated as non-local. The verdict only affects ordering — every
	// address is still dialed — so guessing wrong is cheap.
	localResolveTimeout = 200 * time.Millisecond
	localResolveTTL     = time.Minute
	localCacheMaxSize   = 128
)

type localVerdict struct {
	local   bool
	expires time.Time
}

// localAddrDetector reports whether an address points at the local network:
// loopback, RFC1918/ULA private ranges, or link-local. Hostnames are resolved
// with a short deadline and the verdict is cached.
type localAddrDetector struct {
	mu      sync.Mutex
	cache   map[string]localVerdict
	resolve func(ctx context.Context, host string) ([]net.IPAddr, error)
	now     func() time.Time
}

func newLocalAddrDetector() *localAddrDetector {
	return &localAddrDetector{
		cache:   map[string]localVerdict{},
		resolve: net.DefaultResolver.LookupIPAddr,
		now:     time.Now,
	}
}

func isLocalIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

// isLocal takes a hostport (scheme already stripped) and classifies its host.
func (d *localAddrDetector) isLocal(ctx context.Context, hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	if ip := net.ParseIP(host); ip != nil {
		return isLocalIP(ip)
	}

	now := d.now()
	d.mu.Lock()
	if v, ok := d.cache[host]; ok && now.Before(v.expires) {
		d.mu.Unlock()
		return v.local
	}
	d.mu.Unlock()

	rctx, cancel := context.WithTimeout(ctx, localResolveTimeout)
	defer cancel()
	var local bool
	if ips, err := d.resolve(rctx, host); err == nil {
		for _, ip := range ips {
			if isLocalIP(ip.IP) {
				local = true
				break
			}
		}
	}

	d.mu.Lock()
	if len(d.cache) >= localCacheMaxSize {
		d.cache = map[string]localVerdict{}
	}
	d.cache[host] = localVerdict{local: local, expires: now.Add(localResolveTTL)}
	d.mu.Unlock()
	return local
}
