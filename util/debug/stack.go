package debug

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"runtime"
)

func StackCompact(allGoroutines bool) string {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write(Stack(allGoroutines))
	_ = gz.Close()

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func Stack(allGoroutines bool) []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, allGoroutines)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}
