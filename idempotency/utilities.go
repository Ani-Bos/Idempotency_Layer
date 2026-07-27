package idempotency

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
)

func fingerprintRequest(r *http.Request) string {
	hash_fxn := sha256.New()
	hash_fxn.Write([]byte(r.Method))
	hash_fxn.Write([]byte(r.URL.Path))
	hash_fxn.Write([]byte(r.URL.RawQuery))
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))
	hash_fxn.Write(body)
	return hex.EncodeToString(hash_fxn.Sum(nil))
}