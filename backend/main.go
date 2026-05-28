package main

import (
	"database/sql"
	"door-lock-system/config"
	"door-lock-system/handlers"
	"log"
	"net/http"
	"time"
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

	// Device registration report: ESP posts UID when in registration mode
	mux.HandleFunc("/api/device/register-report", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handlers.RegisterReportHandler(db, w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Registration polling for ESP: return pending mode (normal/admin) or empty
	mux.HandleFunc("/api/registration/pending-mode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			handlers.GetRegistrationModeHandler(db, w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 4. Jalankan HTTP server
	// Start weekly reset goroutine
	go startWeeklyReset(db)

	log.Println("🚀 Server running on :8081")
	if err := http.ListenAndServe(":8081", mux); err != nil {
		log.Fatal("Server error:", err)
	}
}

// startWeeklyReset: Wipe schedules every Monday (once per week) and record last reset date in settings.
func startWeeklyReset(db *sql.DB) {
	for {
		now := time.Now()
		// compute next check at next day's 00:05
		next := time.Date(now.Year(), now.Month(), now.Day(), 0, 5, 0, 0, now.Location()).Add(24 * time.Hour)
		sleep := time.Until(next)
		if sleep < 0 {
			sleep = 1 * time.Minute
		}
		time.Sleep(sleep)

		today := time.Now()
		if today.Weekday() == time.Monday {
			// Check last reset
			var lastReset string
			_ = db.QueryRow("SELECT setting_value FROM settings WHERE setting_key = 'last_weekly_reset'").Scan(&lastReset)
			todayStr := today.Format("2006-01-02")
			if lastReset != todayStr {
				// Perform wipe
				if _, err := db.Exec("DELETE FROM schedules"); err != nil {
					log.Println("❌ Failed to wipe schedules:", err)
				} else {
					log.Println("✅ Weekly schedules wiped (Monday):", todayStr)
					// record last reset
					_, _ = db.Exec("INSERT INTO settings (setting_key, setting_value) VALUES ('last_weekly_reset', ?) ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value)", todayStr)

					// Try notify via Telegram if configured
					if cfg, err := handlers.GetTelegramConfig(db); err == nil && cfg.Enabled {
						_ = handlers.KirimNotifikasi(cfg, "🧹 Jadwal mingguan telah di-reset. Silakan konfigurasi ulang jadwal untuk minggu ini.")
					}
				}
			}
		}
	}
}
