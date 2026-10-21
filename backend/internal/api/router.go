package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"skribbl-dare-backend/internal/auth"
	"skribbl-dare-backend/internal/config"
	"skribbl-dare-backend/internal/database"
	"skribbl-dare-backend/internal/ws"
)

func NewRouter(cfg *config.Config, db *database.MongoDB, hub *ws.Hub) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Cross-Origin Resource Sharing for browser extension and skribbl.io
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"}, // Permissive for browser extension and skribbl.io
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	authHandler := NewAuthHandler(cfg, db)
	dareHandler := NewDareHandler(db, hub)
	roomHandler := NewRoomHandler(cfg, db, hub)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"skribbl-dare-api"}`))
	})

	// Public Dares endpoints
	r.Get("/api/dares/random", withOptionalAuth(cfg.JWTSecret, dareHandler.GetRandomDare))
	r.Get("/api/dares", dareHandler.GetAllDares)
	r.Get("/api/dares/tiers", dareHandler.GetTiers)
	r.Get("/api/leaderboard", dareHandler.GetLeaderboard)
	r.Post("/api/dares/start-round", dareHandler.StartRoundDare)
	r.Post("/api/dares/vote", dareHandler.VoteDare)
	r.Get("/api/dares/current-round/{code}", dareHandler.GetCurrentRoundDare)

	// Rooms & WebSocket
	r.Post("/api/rooms/join", withOptionalAuth(cfg.JWTSecret, roomHandler.JoinRoom))
	r.Get("/api/rooms/{code}/players", roomHandler.GetRoomPlayers)
	r.Get("/ws", roomHandler.HandleWebSocket)

	// Auth routes
	r.Get("/api/auth/discord/login", authHandler.DiscordLogin)
	r.Get("/api/auth/discord/callback", authHandler.DiscordCallback)
	r.Post("/api/auth/dev-login", authHandler.DevLogin)

	// Protected routes
	r.Group(func(protected chi.Router) {
		protected.Use(authMiddleware(cfg.JWTSecret))
		protected.Get("/api/auth/me", authHandler.GetMe)
		protected.Post("/api/dares/complete", dareHandler.CompleteDare)
	})

	return r
}

func authMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractToken(r)
			if tokenStr == "" {
				http.Error(w, `{"error":"Authorization token required"}`, http.StatusUnauthorized)
				return
			}

			claims, err := auth.ValidateToken(tokenStr, jwtSecret)
			if err != nil {
				http.Error(w, `{"error":"Invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), "claims", claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func withOptionalAuth(jwtSecret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractToken(r)
		if tokenStr != "" {
			if claims, err := auth.ValidateToken(tokenStr, jwtSecret); err == nil {
				ctx := context.WithValue(r.Context(), "claims", claims)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	}
}

func extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	if queryToken := r.URL.Query().Get("token"); queryToken != "" {
		return queryToken
	}
	return ""
}
