package commonspace

// PullEventKind tells which stage of a remote space pull a PullEvent
// describes. Consumers must ignore kinds they do not recognize: new kinds may
// be added and are not a breaking change.
type PullEventKind int

const (
	// PullEventUnknown is the zero value and is never emitted; it exists so
	// a zero PullEvent cannot masquerade as a real one.
	PullEventUnknown PullEventKind = iota
	// PullEventWaiting: the pull is about to look for a responsible peer;
	// emitted exactly once per remote pull, before the lookup, which may
	// block. When the lookup never yields a peer — the pull context dies, or
	// the lookup fails outright — no PullEventAttempt is ever made and no
	// terminal event is emitted: the pull's overall outcome is NewSpace's
	// return value.
	PullEventWaiting
	// PullEventAttempt: SpacePull is about to be tried against PeerId.
	PullEventAttempt
	// PullEventResult: the outcome of that attempt. Err is nil on success;
	// otherwise it is the remote RPC failure or a local failure persisting
	// the received space (spacestorage errors) — discriminate by error value
	// if it matters.
	PullEventResult
)

func (k PullEventKind) String() string {
	switch k {
	case PullEventWaiting:
		return "Waiting"
	case PullEventAttempt:
		return "Attempt"
	case PullEventResult:
		return "Result"
	default:
		return "Unknown"
	}
}

// PullEvent describes one stage of fetching a space absent from local
// storage.
type PullEvent struct {
	Kind    PullEventKind
	SpaceId string
	// PeerId is set on Attempt and Result events.
	PeerId string
	// Err is set on Result events; nil means the pull succeeded.
	Err error
}

// PullObserver receives advisory notifications about SpacePull progress while
// fetching a space absent from local storage. It exists for
// status/diagnostics surfaces; the calls never affect the pull.
//
// Several spaces can be pulled concurrently through one observer, so
// implementations must be safe for concurrent use. They must be fast and must
// not block; panics are recovered and logged by the caller. A nil observer is
// allowed.
type PullObserver interface {
	ObservePullEvent(ev PullEvent)
}
