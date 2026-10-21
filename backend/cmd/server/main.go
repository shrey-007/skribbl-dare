package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"skribbl-dare-backend/internal/api"
	"skribbl-dare-backend/internal/config"
	"skribbl-dare-backend/internal/database"
	"skribbl-dare-backend/internal/ws"
)

func main() {
	log.Println("🚀 Starting Skribbl Dare Backend...")

	cfg := config.LoadConfig()

	// Connect to MongoDB
	db, err := database.ConnectMongo(cfg.MongoURI, cfg.DatabaseName)
	if err != nil {
		log.Fatalf("❌ Failed to connect to MongoDB: %v\nMake sure mongod is running on %s", err, cfg.MongoURI)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.Client.Disconnect(ctx)
	}()

	// Initialize WebSocket Hub
	hub := ws.NewHub()
	go hub.Run()

	// Initialize Router
	router := api.NewRouter(cfg, db, hub)

	serverAddr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server run in background
	go func() {
		log.Printf("✨ Skribbl Dare Server listening on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ ListenAndServe failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("⏳ Shutting down server gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("⚠️ Server shutdown error: %v", err)
	}
	log.Println("👋 Server stopped cleanly.")
}
