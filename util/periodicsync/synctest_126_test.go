//go:build go1.26

package periodicsync

import (
	"testing"
	"testing/synctest"
)

func runSyncTest(t *testing.T, f func(*testing.T)) {
	synctest.Test(t, f)
}
