package ldiff

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDiffContainer(t *testing.T) {
	t.Run("when initial read cant set", func(t *testing.T) {
		cont := NewDiffContainer(16, 16)
		initial := cont.InitialDiff().(*olddiff)
		initial.mu.RLock()
		wasWritten := atomic.Bool{}
		go func() {
			cont.Set(Element{Id: "1"})
			wasWritten.Store(true)
		}()
		time.Sleep(10 * time.Millisecond)
		if wasWritten.Load() {
			require.Fail(t, "should not be written")
		}
		initial.mu.RUnlock()
		time.Sleep(10 * time.Millisecond)
		if !wasWritten.Load() {
			require.Fail(t, "should be written")
		}
	})
	t.Run("when precalculated lock cant read initial", func(t *testing.T) {
		cont := NewDiffContainer(16, 16)
		precalc := cont.PrecalculatedDiff().(*diff)
		precalc.mu.Lock()
		wasRead := atomic.Bool{}
		go func() {
			cont.InitialDiff().Len()
			wasRead.Store(true)
		}()
		time.Sleep(10 * time.Millisecond)
		if wasRead.Load() {
			require.Fail(t, "should not be read")
		}
		precalc.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		if !wasRead.Load() {
			require.Fail(t, "should be read")
		}
	})
}
