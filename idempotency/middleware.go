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
		  ctx := r.Context()

		  cached_response, flag, err := cfg.Store.Get(ctx,key)
		  if err == nil && flag {
			for k,v := range cached_response.Headers {
				for _, vv := range v {
					w.Header().Add(k,vv)
				}
		    }
			w.WriteHeader(cached_response.StatusCode)
			w.Write(cached_response.Body)
			return
		}

		locked,err := cfg.Store.Lock(ctx,key,cfg.TTL)
		if err!=nil{
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if !locked {
			http.Error(w, "Request is already in progress", http.StatusConflict)
			return
		}

		recorder := NewResponseRecorder(w)
		next.ServeHTTP(recorder,r)

		if recorder.StatusCode >=500 {
			cfg.Store.Unlock(ctx,key)
			return
		}
        
		response := &Response{
			StatusCode: recorder.StatusCode,
			Body: recorder.Body.Bytes(),
			Headers: recorder.Header(),
		}
		cfg.Store.Set(ctx,key,response,cfg.TTL)
		})
	}
}