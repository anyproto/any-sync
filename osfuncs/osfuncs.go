//go:build !vfs

package osfuncs

import (
	"io/fs"
	"os"
)

var (
	MkdirAll  = os.MkdirAll
	Stat      = os.Stat
	ReadFile  = os.ReadFile
	WriteFile = os.WriteFile
	Remove    = os.Remove
)

// OSFuncs holds replaceable OS-level operations for browser/WASM environments.
type OSFuncs struct {
	MkdirAll  func(path string, perm fs.FileMode) error
	Stat      func(name string) (fs.FileInfo, error)
	ReadFile  func(name string) ([]byte, error)
	WriteFile func(name string, data []byte, perm fs.FileMode) error
	Remove    func(name string) error
}

// SetOSFuncs panics unless built with -tags vfs.
func SetOSFuncs(_ OSFuncs) {
	panic("osfuncs: SetOSFuncs requires building with -tags vfs")
}

// ResetOSFuncs panics unless built with -tags vfs.
func ResetOSFuncs() {
	panic("osfuncs: ResetOSFuncs requires building with -tags vfs")
}
