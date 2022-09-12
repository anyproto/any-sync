package tests

import (
	"bytes"
	"math/rand"
	"testing"
	"time"
)

func BenchmarkHashes(b *testing.B) {
	genRandomBytes := func() [][]byte {
		var res [][]byte
		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)
		for i := 0; i < 10000; i++ {
			var newBytes []byte
			for j := 0; j < 64; j++ {
				newBytes = append(newBytes, byte(r.Intn(256)))
			}
			res = append(res, newBytes)
		}
		return res
	}
	makeStrings := func(input [][]byte) []string {
		var res []string
		for _, bytes := range input {
			res = append(res, string(bytes))
		}
		return res
	}
	res := genRandomBytes()
	stringRes := makeStrings(res)
	stringMap := map[string]struct{}{}
	b.Run("string bytes hash map write", func(b *testing.B) {
		stringMap = map[string]struct{}{}
		for i := 0; i < b.N; i++ {
			for _, bytes := range res {
				stringMap[string(bytes)] = struct{}{}
			}
		}
		b.Run("hash map read", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, bytes := range res {
					_, _ = stringMap[string(bytes)]
				}
			}
		})
	})
	b.Run("compare byte slices as strings", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, bytes := range res {
				if string(bytes) == string(bytes) {

				}
			}
		}
	})
	b.Run("compare byte slices", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, bt := range res {
				if bytes.Compare(bt, bt) == 0 {

				}
			}
		}
	})
	b.Run("compare strings", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, st := range stringRes {
				if st == st {

				}
			}
		}
	})
	b.Run("string hash map write", func(b *testing.B) {
		stringMap = map[string]struct{}{}
		for i := 0; i < b.N; i++ {
			for _, str := range stringRes {
				stringMap[str] = struct{}{}
			}
		}
		b.Run("hash map read", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, str := range stringRes {
					_, _ = stringMap[str]
				}
			}
		})
	})
}
