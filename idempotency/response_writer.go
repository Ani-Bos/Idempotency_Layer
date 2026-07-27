package idempotency

import(
	"bytes"
	"net/http"
)

type ResponseRecorder struct {
	http.ResponseWriter
	StatusCode int
	Body    *bytes.Buffer
}

func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		ResponseWriter: w,
		StatusCode: http.StatusOK,
		Body:    &bytes.Buffer{},
	}
}

func (r *ResponseRecorder) WriteHeader(statusCode int) {
	r.StatusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *ResponseRecorder) Write(b []byte) (int, error) {
	r.Body.Write(b)
	return r.ResponseWriter.Write(b)
}