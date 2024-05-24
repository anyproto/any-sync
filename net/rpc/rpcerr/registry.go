package rpcerr

import (
	"errors"
	"fmt"

	"storj.io/drpc/drpcerr"
)

var (
	Unexpected = RegisterErr(errors.New("unexpected"), 1)
	Closed     = RegisterErr(errors.New("closed"), 2)
)

var (
	errsMap = make(map[uint64]error)
)

func RegisterErr(err error, code uint64) error {
	if e, ok := errsMap[code]; ok {
		panic(fmt.Errorf("attempt to register error with existing code: %d; registered error: %v", code, e))
	}
	errWithCode := drpcerr.WithCode(err, code)
	errsMap[code] = errWithCode
	return errWithCode
}

func Code(err error) uint64 {
	return drpcerr.Code(err)
}

func Err(code uint64) error {
	err, ok := errsMap[code]
	if !ok {
		return drpcerr.WithCode(fmt.Errorf("unexpected error, code: %d", code), code)
	}
	return err
}

func Unwrap(e error) error {
	code := drpcerr.Code(e)
	if code == 0 {
		return e
	}
	err, ok := errsMap[code]
	if !ok {
		return drpcerr.WithCode(fmt.Errorf("unexpected error: %w; code: %d", e, code), code)
	}
	return err
}

type ErrGroup int64

func (g ErrGroup) Register(err error, code uint64) error {
	return RegisterErr(err, uint64(g)+code)
}
