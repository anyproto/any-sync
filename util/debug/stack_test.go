package debug

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStack(t *testing.T) {
	stack := Stack(true)
	require.True(t, strings.Contains(string(stack), "main.main"))
}

func TestStackCompact(t *testing.T) {
	stack := StackCompact(true)
	decoded, err := base64.StdEncoding.DecodeString(string(stack))
	require.NoError(t, err)
	rd, err := gzip.NewReader(bytes.NewReader(decoded))
	require.NoError(t, err)
	var (
		buf = make([]byte, 1024)
		res []byte
	)
	for {
		n, err := rd.Read(buf)
		if n > 0 {
			res = append(res, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	require.True(t, strings.Contains(string(res), "main.main"))
}
