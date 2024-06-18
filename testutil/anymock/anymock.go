package anymock

import "go.uber.org/mock/gomock"

type MockComp interface {
	Name() *gomock.Call
	Init(x any) *gomock.Call
}

type MockCompRunnable interface {
	Run(x any) *gomock.Call
	Close(x any) *gomock.Call
}

func ExpectComp(c MockComp, name string) {
	c.Name().Return(name).AnyTimes()
	c.Init(gomock.Any()).AnyTimes()
	if cr, ok := c.(MockCompRunnable); ok {
		cr.Run(gomock.Any()).AnyTimes()
		cr.Close(gomock.Any()).AnyTimes()
	}
}
