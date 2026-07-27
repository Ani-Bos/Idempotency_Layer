package idempotency

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIdempotentReplay(t *testing.T) {
	store := NewMemoryStore(100)
	mw := NewMiddleware(Config{Store: store, TTL: time.Hour})

	var calls int32
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("X-Custom", "yes")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"order-123"}`))
	}))

	body := strings.NewReader(`{"amount":100}`)
	req1 := httptest.NewRequest(http.MethodPost, "/order", body)
	req1.Header.Set("Idempotency-Key", "key-replay")

	body2 := strings.NewReader(`{"amount":100}`)
	req2 := httptest.NewRequest(http.MethodPost, "/order", body2)
	req2.Header.Set("Idempotency-Key", "key-replay")

	rr1 := httptest.NewRecorder()
	rr2 := httptest.NewRecorder()

	handler.ServeHTTP(rr1, req1)
	handler.ServeHTTP(rr2, req2)

	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("handler called %d times, want 1", calls)
	}
	if rr2.Code != http.StatusCreated {
		t.Fatalf("replay status = %d, want 201", rr2.Code)
	}
	if rr2.Body.String() != `{"id":"order-123"}` {
		t.Fatalf("replay body = %q", rr2.Body.String())
	}
}

func TestConcurrentRetryBlocksNot409(t *testing.T) {
	store := NewMemoryStore(100)
	mw := NewMiddleware(Config{Store: store, TTL: time.Hour})

	var calls int32
	block := make(chan struct{})

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		<-block
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("done"))
	}))

	body1 := strings.NewReader(`{"amount":100}`)
	req1 := httptest.NewRequest(http.MethodPost, "/order", body1)
	req1.Header.Set("Idempotency-Key", "key-concurrent")

	body2 := strings.NewReader(`{"amount":100}`)
	req2 := httptest.NewRequest(http.MethodPost, "/order", body2)
	req2.Header.Set("Idempotency-Key", "key-concurrent")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req1)
	}()

	time.Sleep(50 * time.Millisecond)

	var retryCode int
	var retryBody string
	go func() {
		defer wg.Done()
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req2)
		retryCode = rr.Code
		retryBody = rr.Body.String()
	}()

	time.Sleep(100 * time.Millisecond)
	close(block)
	wg.Wait()

	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("handler called %d times, want 1", calls)
	}
	if retryCode == http.StatusConflict {
		t.Fatal("retry got 409; should have blocked and replayed")
	}
	if retryBody != "done" {
		t.Fatalf("retry body = %q, want done", retryBody)
	}
}

func TestFingerprintMismatch(t *testing.T) {
	store := NewMemoryStore(100)
	mw := NewMiddleware(Config{Store: store, TTL: time.Hour})

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	body1 := strings.NewReader(`{"amount":100}`)
	req1 := httptest.NewRequest(http.MethodPost, "/order", body1)
	req1.Header.Set("Idempotency-Key", "key-mismatch")

	body2 := strings.NewReader(`{"amount":200}`)
	req2 := httptest.NewRequest(http.MethodPost, "/order", body2)
	req2.Header.Set("Idempotency-Key", "key-mismatch")

	rr1 := httptest.NewRecorder()
	rr2 := httptest.NewRecorder()

	handler.ServeHTTP(rr1, req1)
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Fatalf("mismatch status = %d, want 409", rr2.Code)
	}
}

func TestTransientFailureNotCached(t *testing.T) {
	store := NewMemoryStore(100)
	mw := NewMiddleware(Config{Store: store, TTL: time.Hour})

	var calls int32
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	}))

	body1 := strings.NewReader(`{"amount":100}`)
	req1 := httptest.NewRequest(http.MethodPost, "/order", body1)
	req1.Header.Set("Idempotency-Key", "key-500")

	body2 := strings.NewReader(`{"amount":100}`)
	req2 := httptest.NewRequest(http.MethodPost, "/order", body2)
	req2.Header.Set("Idempotency-Key", "key-500")

	rr1 := httptest.NewRecorder()
	rr2 := httptest.NewRecorder()

	handler.ServeHTTP(rr1, req1)
	handler.ServeHTTP(rr2, req2)

	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("handler called %d times, want 2", calls)
	}
}

func TestClientErrorCached(t *testing.T) {
	store := NewMemoryStore(100)
	mw := NewMiddleware(Config{Store: store, TTL: time.Hour})

	var calls int32
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte("invalid"))
	}))

	body1 := strings.NewReader(`{"amount":100}`)
	req1 := httptest.NewRequest(http.MethodPost, "/order", body1)
	req1.Header.Set("Idempotency-Key", "key-422")

	body2 := strings.NewReader(`{"amount":100}`)
	req2 := httptest.NewRequest(http.MethodPost, "/order", body2)
	req2.Header.Set("Idempotency-Key", "key-422")

	rr1 := httptest.NewRecorder()
	rr2 := httptest.NewRecorder()

	handler.ServeHTTP(rr1, req1)
	handler.ServeHTTP(rr2, req2)

	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("handler called %d times, want 1", calls)
	}
	if rr2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("replay status = %d, want 422", rr2.Code)
	}
}

func TestNoKeyPassthrough(t *testing.T) {
	store := NewMemoryStore(100)
	mw := NewMiddleware(Config{Store: store, TTL: time.Hour})

	var calls int32
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Write([]byte("ok"))
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/order", strings.NewReader("body"))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("handler called %d times, want 3", calls)
	}
}

func TestPanicCleanup(t *testing.T) {
	store := NewMemoryStore(100)
	mw := NewMiddleware(Config{Store: store, TTL: time.Hour})

	panicHandler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	body1 := strings.NewReader(`{"amount":100}`)
	req1 := httptest.NewRequest(http.MethodPost, "/order", body1)
	req1.Header.Set("Idempotency-Key", "key-panic")

	func() {
		defer func() { recover() }()
		rr := httptest.NewRecorder()
		panicHandler.ServeHTTP(rr, req1)
	}()

	var calls int32
	normalHandler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("recovered"))
	}))

	body2 := strings.NewReader(`{"amount":100}`)
	req2 := httptest.NewRequest(http.MethodPost, "/order", body2)
	req2.Header.Set("Idempotency-Key", "key-panic")

	rr2 := httptest.NewRecorder()
	normalHandler.ServeHTTP(rr2, req2)

	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("retry after panic called %d times, want 1", calls)
	}
}

func TestMemoryBounded(t *testing.T) {
	store := NewMemoryStore(2)
	mw := NewMiddleware(Config{Store: store, TTL: time.Hour})

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	}))

	for i := 0; i < 5; i++ {
		body := strings.NewReader(`{"amount":100}`)
		req := httptest.NewRequest(http.MethodPost, "/order", body)
		req.Header.Set("Idempotency-Key", "key-"+string(rune('a'+i)))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
	
	store.mu.Lock()
	if store.lruList.Len() > 2 {
		t.Fatalf("store size = %d, want <= 2", store.lruList.Len())
	}
	store.mu.Unlock()
}

func TestRace(t *testing.T) {
	store := NewMemoryStore(1000)
	mw := NewMiddleware(Config{Store: store, TTL: time.Hour})

	var calls int32
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	}))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				body := strings.NewReader(`{"amount":100}`)
				req := httptest.NewRequest(http.MethodPost, "/order", body)
				req.Header.Set("Idempotency-Key", "race-key")
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
			}
		}()
	}
	wg.Wait()

	if atomic.LoadInt32(&calls) < 1 {
		t.Fatal("handler was never called")
	}
}

func TestBodyAvailableToHandler(t *testing.T) {
	store := NewMemoryStore(100)
	mw := NewMiddleware(Config{Store: store, TTL: time.Hour})

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Write(body)
	}))

	body := strings.NewReader(`{"amount":100}`)
	req := httptest.NewRequest(http.MethodPost, "/order", body)
	req.Header.Set("Idempotency-Key", "key-body")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Body.String() != `{"amount":100}` {
		t.Fatalf("handler saw body = %q", rr.Body.String())
	}
}