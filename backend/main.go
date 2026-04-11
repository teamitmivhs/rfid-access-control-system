package main

import (
	"door-lock-system/config"
	"door-lock-system/handlers"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// JSON response helper
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func main() {
	db := config.InitDB()
	defer db.Close()

	// Setup Telegram Bot (run in goroutine)
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken != "" {
		go func() {
			if err := handlers.StartTelegramBot(db, botToken); err != nil {
				log.Printf("Telegram Bot Error: %v\n", err)
			}
		}()
		log.Println("Telegram Bot started")
	} else {
		log.Println("⚠️  TELEGRAM_BOT_TOKEN not set - Bot disabled")
	}

	// Setup routes
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/access/verify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}
		handlers.VerifyAccessHandler(db, w, r)
	})

	// Wrap dengan CORS middleware
	httpHandler := corsMiddleware(mux)

	// Start server
	addr := ":8080"
	log.Printf("Server running on %s\n", addr)
	if err := http.ListenAndServe(addr, httpHandler); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}
