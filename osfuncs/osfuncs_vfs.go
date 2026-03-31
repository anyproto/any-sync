//go:build vfs

package osfuncs

import (
	"io/fs"
	"os"
)

var (
	MkdirAll  func(path string, perm fs.FileMode) error             = os.MkdirAll
	Stat      func(name string) (fs.FileInfo, error)                = os.Stat
	ReadFile  func(name string) ([]byte, error)                     = os.ReadFile
	WriteFile func(name string, data []byte, perm fs.FileMode) error = os.WriteFile
	Remove    func(name string) error                               = os.Remove
)

// OSFuncs holds replaceable OS-level operations for browser/WASM environments.
type OSFuncs struct {
	MkdirAll  func(path string, perm fs.FileMode) error
	Stat      func(name string) (fs.FileInfo, error)
	ReadFile  func(name string) ([]byte, error)
	WriteFile func(name string, data []byte, perm fs.FileMode) error
	Remove    func(name string) error
}

// SetOSFuncs replaces OS-level operations at runtime. Nil fields keep defaults.
func SetOSFuncs(fns OSFuncs) {
	if fns.MkdirAll != nil {
		MkdirAll = fns.MkdirAll
	}
	if fns.Stat != nil {
		Stat = fns.Stat
	}
	if fns.ReadFile != nil {
		ReadFile = fns.ReadFile
	}
	if fns.WriteFile != nil {
		WriteFile = fns.WriteFile
	}
	if fns.Remove != nil {
		Remove = fns.Remove
	}
}

// ResetOSFuncs restores all operations to their OS defaults.
func ResetOSFuncs() {
	MkdirAll = os.MkdirAll
	Stat = os.Stat
	ReadFile = os.ReadFile
	WriteFile = os.WriteFile
	Remove = os.Remove
}
