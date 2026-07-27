package main

import (
	"net/http"
	"time"
	"vectorshift_assignment/idempotency"
	"fmt"
)
func main(){
	mem_store := idempotency.NewMemoryStore(1000)

	middleware := idempotency.NewMiddleware(idempotency.Config{
		Store: mem_store,
		TTL: 5 * time.Minute,
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(">>> Handler executed for", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"created","id":"order-123"}` + "\n"))
	}))
	fmt.Println("Server listening on :8080")
	http.ListenAndServe(":8080", handler)
}