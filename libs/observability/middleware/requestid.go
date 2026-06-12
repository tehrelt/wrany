package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/wrany/libs/observability/logger"
)

const headerRequestID = "X-Request-Id"

// RequestID reads X-Request-Id from the incoming request (if present) or
// generates a new UUID, injects it into the request context, and echoes it
// in the response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(headerRequestID)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(headerRequestID, id)
		ctx := logger.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
