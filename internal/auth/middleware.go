// backend/internal/auth/middleware.go
package auth

import (
    "context"
    "net/http"
    "strings"
    "github.com/dgrijalva/jwt-go"
)

// backend/internal/auth/middleware.go
func JWTMiddleware(jwtSecret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Authorization header required", http.StatusUnauthorized)
                return
            }

            bearerToken := strings.Split(authHeader, " ")
            if len(bearerToken) != 2 || bearerToken[0] != "Bearer" {
                http.Error(w, "Invalid token format", http.StatusUnauthorized)
                return
            }

            token, err := jwt.ParseWithClaims(bearerToken[1], &jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
                return []byte(jwtSecret), nil
            })

            if err != nil {
                http.Error(w, "Invalid token", http.StatusUnauthorized)
                return
            }

            claims, ok := token.Claims.(*jwt.MapClaims)
            if !ok || !token.Valid {
                http.Error(w, "Invalid token claims", http.StatusUnauthorized)
                return
            }

            userID, ok := (*claims)["user_id"].(float64)
            if !ok {
                http.Error(w, "Invalid user ID in token", http.StatusUnauthorized)
                return
            }

            ctx := context.WithValue(r.Context(), "user_id", uint(userID))
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}