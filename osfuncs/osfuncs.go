package osfuncs

import "os"

type OSFuncs struct {
	MkdirAll  func(path string, perm os.FileMode) error
	Stat      func(name string) (os.FileInfo, error)
	ReadFile  func(name string) ([]byte, error)
	WriteFile func(name string, data []byte, perm os.FileMode) error
	Remove    func(name string) error
}

var (
	osMkdirAll  = os.MkdirAll
	osStat      = os.Stat
	osReadFile  = os.ReadFile
	osWriteFile = os.WriteFile
	osRemove    = os.Remove
)

func SetOSFuncs(funcs OSFuncs) {
	if funcs.MkdirAll != nil {
		osMkdirAll = funcs.MkdirAll
	}
	if funcs.Stat != nil {
		osStat = funcs.Stat
	}
	if funcs.ReadFile != nil {
		osReadFile = funcs.ReadFile
	}
	if funcs.WriteFile != nil {
		osWriteFile = funcs.WriteFile
	}
	if funcs.Remove != nil {
		osRemove = funcs.Remove
	}
}

func ResetOSFuncs() {
	osMkdirAll = os.MkdirAll
	osStat = os.Stat
	osReadFile = os.ReadFile
	osWriteFile = os.WriteFile
	osRemove = os.Remove
}

func MkdirAll(path string, perm os.FileMode) error               { return osMkdirAll(path, perm) }
func Stat(name string) (os.FileInfo, error)                       { return osStat(name) }
func ReadFile(name string) ([]byte, error)                        { return osReadFile(name) }
func WriteFile(name string, data []byte, perm os.FileMode) error  { return osWriteFile(name, data, perm) }
func Remove(name string) error                                    { return osRemove(name) }
