package idempotency

import(
	"bytes"
	"net/http"
)

type ResponseRecorder struct {
	http.ResponseWriter
	StatusCode int
	Body    *bytes.Buffer
	wroteHeader bool
}

func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		ResponseWriter: w,
		StatusCode: http.StatusOK,
		Body:    &bytes.Buffer{},
	}
}

func (r *ResponseRecorder) WriteHeader(statusCode int) {
	if r.wroteHeader{
		return
	}
	r.wroteHeader = true
	r.StatusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *ResponseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	r.Body.Write(b)
	return r.ResponseWriter.Write(b)
}