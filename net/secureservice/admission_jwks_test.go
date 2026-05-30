package secureservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchAdmissionJWKSHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		_, err := w.Write([]byte(`{"keys":[]}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	jwks, err := fetchAdmissionJWKSHTTP(context.Background(), server.URL)

	require.NoError(t, err)
	assert.JSONEq(t, `{"keys":[]}`, string(jwks))
}

func TestFetchAdmissionJWKSHTTPRejectsNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := fetchAdmissionJWKSHTTP(context.Background(), server.URL)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestFetchAdmissionJWKSHTTPRejectsLargeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(strings.Repeat("a", maxAdmissionJWKSBytes+1)))
		require.NoError(t, err)
	}))
	defer server.Close()

	_, err := fetchAdmissionJWKSHTTP(context.Background(), server.URL)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}
