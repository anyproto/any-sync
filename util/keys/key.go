package keys

type Key interface {
	Equals(Key) bool

	Raw() ([]byte, error)
}
