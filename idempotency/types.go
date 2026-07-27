package idempotency

import (
	"context"
	"time"
	"net/http"

)
type Response struct{
	StatusCode int
	Body 	 []byte
	Headers	http.Header
}

type Store interface {
	Set(ctx context.Context, key string, response *Response , ttl time.Duration) error
	Get(ctx context.Context, key string) (*Response, bool,error)
	Lock(ctx context.Context, key string , ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key string) error
}
