package middleware

import (
	"context"
	"github.com/serlenario/caching-webserver/internal/storage"
	"net/http"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("token")
		if token == "" {
			http.Error(w, `{"error": {"code": 401, "text": "Unauthorized"}}`, http.StatusUnauthorized)
			return
		}

		cache := storage.GetCache()
		userID, found := cache.Get(token)
		if !found {
			http.Error(w, `{"error": {"code": 401, "text": "Unauthorized"}}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "userID", userID.(int))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
