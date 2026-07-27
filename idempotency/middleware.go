package idempotency
import (
	"net/http"
	"time"
)

type Config struct {
	HeaderName string
	Store      Store
	TTL        time.Duration
}

func NewMiddleware(cfg Config) func (http.Handler) http.Handler {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "Idempotency-Key"
	}
	if cfg.TTL == 0 {
		cfg.TTL = 24*time.Hour
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          key := r.Header.Get(cfg.HeaderName)
		  if key=="" || r.Method == http.MethodGet || r.Method == http.MethodOptions{
			 next.ServeHTTP(w,r)
			 return
		  }
		  fp:=fingerprintRequest(r)
		  ctx := r.Context()

	      status, cached, waitCh, err := cfg.Store.Start(ctx, key, fp, cfg.TTL)
			if err != nil {
				if err == ErrFingerPrintMismatch {
					http.Error(w, "idempotency key mismatch occurs", http.StatusConflict)
					return
				}
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}

			if status == StatusHit {
				replay(cached, w)
				return
			} else if status == StatusInProgress {
				resp := <-waitCh
				replay(resp, w)
				return
			}

			rec := NewResponseRecorder(w)
			var result *Response
			shouldCache := false
			defer func() {
				if recov := recover(); recov != nil {
					cfg.Store.Complete(ctx, key, fp, nil, cfg.TTL)
					panic(recov)
				}
				if shouldCache {
					cfg.Store.Complete(ctx, key, fp, result, cfg.TTL)
				} else {
					cfg.Store.Complete(ctx, key, fp, nil, cfg.TTL)
				}
			}()

			next.ServeHTTP(rec, r)
			if rec.StatusCode < 500 {
				shouldCache = true
				result = &Response{
					StatusCode: rec.StatusCode,
					Body:       rec.Body.Bytes(),
					Headers:    rec.Header().Clone(),
				}
			}
		})
	}
}

func replay(r *Response, w http.ResponseWriter) {
	for k, v := range r.Headers {
		for _, value := range v {
			w.Header().Add(k, value)
		}
	}
	w.WriteHeader(r.StatusCode)
	w.Write(r.Body)
}