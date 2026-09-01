// Package peerobserver defines an advisory observer of the peer connection
// lifecycle: dials, established connections in both directions and connection
// deaths. It exists for status/diagnostics surfaces; the notifications never
// affect control flow.
//
// Register an Observer as an app component under CName (use New) before the
// app starts; peerservice and pool then look it up at Init and feed it from
// the first dial. When absent, no events are produced.
package peerobserver

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/any-sync/app/logger"
)

// CName is the app component name under which an Observer is registered.
const CName = "net.peerobserver"

var log = logger.NewNamed(CName)

// Kind tells which lifecycle event an Event describes. Consumers must ignore
// kinds they do not recognize: new kinds may be added and are not a breaking
// change.
type Kind int

const (
	// KindUnknown is the zero value and is never emitted; it exists so a
	// zero Event cannot masquerade as a real one.
	KindUnknown Kind = iota
	// KindDialStarted: an outgoing dial began. Fields: PeerId, AddrCount.
	KindDialStarted
	// KindConnected: a connection was established — outbound (dial
	// succeeded) or inbound (accepted from a listener). Fields: PeerId,
	// Addr, Scheme, Inbound, ProtoVersion, Dur (outbound only).
	KindConnected
	// KindDialFailed: a dial produced no connection. Fields: PeerId, Err,
	// Dur.
	KindDialFailed
	// KindClosed: a connection died or was evicted from the pool. Fields:
	// PeerId, Inbound.
	KindClosed
)

func (k Kind) String() string {
	switch k {
	case KindDialStarted:
		return "DialStarted"
	case KindConnected:
		return "Connected"
	case KindDialFailed:
		return "DialFailed"
	case KindClosed:
		return "Closed"
	default:
		return "Unknown"
	}
}

// Event describes one peer connection lifecycle event. Which fields are set
// depends on Kind; unset fields are zero.
type Event struct {
	Kind Kind
	// PeerId is the remote peer id.
	PeerId string
	// AddrCount is the number of dial candidates after transport filtering
	// and ordering; 0 means no dialable address was known.
	AddrCount int
	// Addr is the connection address without its transport scheme prefix:
	// outbound it is the address that was dialed, which may be a configured
	// hostname; inbound it is whatever the transport reports as the remote
	// address, whose form is transport-specific. The two are not comparable.
	// Treat it as display/debug data; privacy-sensitive forms (an outbound
	// iroh ticket, which encodes relay and IP addresses) arrive shortened.
	Addr string
	// Scheme is the transport scheme, taken from the connection's address.
	// Outbound it is always one of the known schemes; empty only for an
	// inbound connection whose address carried no scheme prefix.
	Scheme string
	// Inbound is true for connections accepted from a listener.
	Inbound bool
	// ProtoVersion is the protocol version negotiated on the handshake, 0 if
	// absent from the connection context.
	ProtoVersion uint32
	// Dur is how long the dial took (success or failure), measured from the
	// start of Dial — it includes address resolution and ordering, so it can
	// exceed the observed DialStarted→terminal interval. 0 for inbound.
	Dur time.Duration
	// Err is why the dial failed. Address dial failures always arrive as an
	// errors.Join of every per-address error (a single failing address still
	// comes joined; unwrap via interface{ Unwrap() []error }, never by
	// parsing the newline-separated message). Two shapes arrive bare: a
	// sentinel such as peerservice.ErrAddrsNotFound when no address was
	// attempted, and post-connection rejections (peer id mismatch, a missing
	// peer id in the connection context, peer construction failure).
	// errors.Is works on every shape.
	Err error
}

// Observer receives advisory notifications about the lifecycle of peer
// connections.
//
// Guarantees:
//   - every peerservice.Dial produces exactly one KindDialStarted followed by
//     exactly one of KindConnected or KindDialFailed;
//   - every pool-managed connection produces one KindClosed when it dies —
//     including a connection the pool refused on accept, whose KindClosed
//     comes right after its KindConnected. Best effort once pool shutdown has
//     begun: from that point KindClosed events are suppressed, and a
//     connection accepted while shutting down may never report one. A peer
//     added straight through pool.AddPeer (bypassing peerservice.Accept)
//     reports KindClosed without a preceding KindConnected, and a peer dialed
//     through peerservice.Dial directly (bypassing the pool) reports
//     KindConnected without ever reporting KindClosed.
//
// The stream reports connections, not per-peer liveness: track it by counting
// open connections per peer (clamping at zero), never by latching a per-peer
// boolean. KindClosed for a superseded connection can arrive after
// KindConnected for its replacement, and the pool closes idle connections on
// a TTL (about a minute), which also produces KindClosed — the event carries
// no reason yet, so a consumer cannot currently distinguish an idle close
// from a network failure. The stream reports dials, not connection attempts:
// after a version-mismatch handshake failure the pool caches the error
// verdict and suppresses re-dials for up to 20 minutes — the first failure is
// observed, subsequent suppressed attempts are not. An inbound connection
// that fails before it becomes a peer produces no event.
//
// Calls arrive concurrently from dial callers, transport accept loops and
// per-connection watcher goroutines; implementations must be safe for
// concurrent use. Dial-path events (KindDialStarted, outbound KindConnected,
// KindDialFailed) run inside the pool's single-flight load for that peer: an
// implementation must never call the pool for the peer such an event names —
// the load is still open and the call blocks until the caller's context dies.
// A pool call for a DIFFERENT peer is allowed, but it can synchronously
// re-enter ObservePeerEvent on the same goroutine (a Get that dials emits
// that peer's dial-path events, recursively subject to the same rule), so
// never hold a non-reentrant lock across one — hand such work to another
// goroutine. KindClosed is delivered outside any load. Implementations must
// be fast and must not block: on the dial path, blocking stalls every Get and
// Pick for that peer. Panics are recovered and logged.
type Observer interface {
	ObservePeerEvent(ev Event)
}

// Notify delivers ev to obs with panic containment: a status surface must
// never break networking. It is a no-op when obs is nil. It is a
// producer-side helper; consumers implement Observer and never call it.
func Notify(obs Observer, ev Event) {
	if obs == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Error("peer observer panic", zap.Any("recover", r))
		}
	}()
	obs.ObservePeerEvent(ev)
}

// Notifier delivers events to the Observer registered under CName. Its zero
// value is a no-op, so a producer can hold and call one unconditionally.
type Notifier struct {
	obs Observer
}

// FromApp returns a Notifier for the Observer registered under CName, or a
// no-op Notifier when none is registered. A component registered under the
// name that does not implement Observer is ignored with a warning.
func FromApp(a *app.App) Notifier {
	comp := a.Component(CName)
	if comp == nil {
		return Notifier{}
	}
	obs, ok := comp.(Observer)
	if !ok {
		log.Warn("component registered under peer observer name does not implement peerobserver.Observer",
			zap.String("type", fmt.Sprintf("%T", comp)))
		return Notifier{}
	}
	return Notifier{obs: obs}
}

// Notify delivers ev with the same containment as the package-level Notify.
func (n Notifier) Notify(ev Event) {
	Notify(n.obs, ev)
}

// New wraps obs into an app component registered under CName, so a consumer
// does not have to write the adapter:
//
//	app.Register(peerobserver.New(myObserver))
//
// A nil obs yields an inert component: registered, but producing no work per
// event. The wrapper is a plain app.Component; an observer that needs
// Run/Close should instead implement app.ComponentRunnable itself with
// Name() returning CName and register directly.
func New(obs Observer) app.Component {
	if obs == nil {
		obs = noopObserver{}
	}
	return &component{Observer: obs}
}

type noopObserver struct{}

func (noopObserver) ObservePeerEvent(Event) {}

type component struct {
	Observer
}

func (c *component) Init(a *app.App) (err error) { return nil }
func (c *component) Name() string                { return CName }
