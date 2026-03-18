package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"CampusMonitorAPI/internal/logger"
)

func Recovery(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error("PANIC: %v", err)
					log.Error("Stack trace:\n%s", debug.Stack())

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, err2 := w.Write([]byte(fmt.Sprintf(`{"error": "Internal server error: %v"}`, err)))
					if err2 != nil {
						return
					}
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
