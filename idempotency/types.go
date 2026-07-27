package idempotency

import (
	"context"
	"errors"
	"net/http"
	"time"
)
type Response struct{
	StatusCode int
	Body 	 []byte
	Headers	http.Header
}

type StartStatus int

const (
	StatusMiss StartStatus = iota
	StatusInProgress
	StatusHit
)
var ErrFingerPrintMismatch = errors.New("idempotency key missmatch after hashing request")
type Store interface {
    Start(ctx context.Context,key string, fingerprint string, ttl time.Duration)(
		status StartStatus,
		cached_response *Response,
		waitCh <-chan *Response,
		err error,
	)
	Complete(ctx context.Context,key string, fingerprint string, resp *Response, ttl time.Duration) error	
}
