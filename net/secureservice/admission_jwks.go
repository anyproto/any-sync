package secureservice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultAdmissionJWKSTimeout = 10 * time.Second
	maxAdmissionJWKSBytes       = 1 << 20
)

var fetchAdmissionJWKS = fetchAdmissionJWKSHTTP

func newAdmissionVerifierFromConfig(conf AdmissionConfig) (AdmissionVerifier, error) {
	conf = conf.WithDefaults()
	if conf.JWKSURL == "" {
		return nil, ErrAdmissionInvalidConfig
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultAdmissionJWKSTimeout)
	defer cancel()

	jwks, err := fetchAdmissionJWKS(ctx, conf.JWKSURL)
	if err != nil {
		return nil, errors.Join(ErrAdmissionInvalidConfig, err)
	}
	verifier, err := NewJWTAdmissionVerifier(conf, jwks)
	if err != nil {
		return nil, errors.Join(ErrAdmissionInvalidConfig, err)
	}
	return verifier, nil
}

func fetchAdmissionJWKSHTTP(ctx context.Context, url string) ([]byte, error) {
	if url == "" {
		return nil, ErrAdmissionInvalidConfig
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch admission jwks: invalid url")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("fetch admission jwks: request failed: %w", ctxErr)
		}
		return nil, fmt.Errorf("fetch admission jwks: request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("fetch admission jwks: %s", resp.Status)
	}
	reader := io.LimitReader(resp.Body, maxAdmissionJWKSBytes+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(data) > maxAdmissionJWKSBytes {
		return nil, fmt.Errorf("admission jwks exceeds %d bytes", maxAdmissionJWKSBytes)
	}
	return data, nil
}
