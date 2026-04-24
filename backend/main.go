package main

import (
	"door-lock-system/config"
	"door-lock-system/handlers"
	"log"
	"net/http"
)

func main() {
	// 1. Koneksi ke database
	db := config.InitDB()
	defer db.Close()

	// 2. Jalankan Telegram bot
	// Token & chat_id dibaca dari tabel settings di database — tidak hardcode di sini
	if err := handlers.StartTelegramBot(db); err != nil {
		// Jika bot gagal start, server tetap jalan (bot tidak wajib)
		log.Println("⚠️  Telegram bot gagal start:", err)
	}

	// 3. Register HTTP routes
	mux := http.NewServeMux()

	// Verifikasi akses kartu RFID dari ESP32
	mux.HandleFunc("/api/access/verify", func(w http.ResponseWriter, r *http.Request) {
		handlers.VerifyAccessHandler(db, w, r)
	})

	// Sync kartu admin + scheduled untuk hari ini (endpoint lama, tetap ada)
	mux.HandleFunc("/api/cards/today", func(w http.ResponseWriter, r *http.Request) {
		handlers.GetCardsForTodayHandler(db, w, r)
	})

	// Sync HANYA scheduled cards untuk hari ini (dipakai ESP32 sekarang)
	mux.HandleFunc("/api/cards/scheduled-today", func(w http.ResponseWriter, r *http.Request) {
		handlers.GetScheduledCardsForTodayHandler(db, w, r)
	})

	// Jadwal akses per user
	mux.HandleFunc("/api/schedule", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.GetScheduleHandler(db, w, r)
		case http.MethodPost:
			handlers.UpdateScheduleHandler(db, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Device heartbeat - status update dari ESP32
	mux.HandleFunc("/api/device/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handlers.DeviceHeartbeatHandler(db, w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 4. Jalankan HTTP server
	log.Println("🚀 Server running on :8081")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		log.Fatal("Server error:", err)
	}
}
