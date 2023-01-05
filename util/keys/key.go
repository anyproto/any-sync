package keys

import "crypto/subtle"

type Key interface {
	Raw() ([]byte, error)
}

func KeyEquals(k1, k2 Key) bool {
	a, err := k1.Raw()
	if err != nil {
		return false
	}
	b, err := k2.Raw()
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}
