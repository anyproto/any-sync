package util

type ReleaseContainer[T any] struct {
	Object  T
	Release func()
}
