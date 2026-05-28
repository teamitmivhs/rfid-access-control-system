package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

// TelegramUpdate: Struktur untuk menerima update dari Telegram
type TelegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  struct {
		MessageID int `json:"message_id"`
		From      struct {
			ID   int    `json:"id"`
			Name string `json:"first_name"`
		} `json:"from"`
		Chat struct {
			ID   int    `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
		Text string `json:"text"`
		Date int64  `json:"date"`
	} `json:"message"`
}

// TelegramResponse: Response untuk kirim ke Telegram
type TelegramResponse struct {
	OK     bool                   `json:"ok"`
	Result map[string]interface{} `json:"result,omitempty"`
	Error  string                 `json:"error_description,omitempty"`
}

// TelegramWebhookHandler: Handle incoming updates dari Telegram
// Endpoint: POST /telegram/webhook
func TelegramWebhookHandler(db *sql.DB, botToken string, w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Failed to read body"})
		return
	}

	var update TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	// Abaikan jika bukan message
	if update.Message.Text == "" {
		jsonResponse(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	text := strings.TrimSpace(update.Message.Text)
	chatID := update.Message.Chat.ID

	// Parse command
	if strings.HasPrefix(text, "/sync") {
		handleSyncCommand(db, botToken, chatID)
		jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if strings.HasPrefix(text, "/status") {
		handleStatusCommand(db, botToken, chatID)
		jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	// Unknown command
	sendTelegramMessage(botToken, chatID, "❓ Command tidak dikenal. Gunakan:\n/sync - Sync kartu dari database\n/status - Lihat status pintu")
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleSyncCommand: Handle /sync command
func handleSyncCommand(db *sql.DB, botToken string, chatID int) {
	// Set sync pending flag di database
	query := `
		INSERT INTO settings (setting_key, setting_value) 
		VALUES ('sync_pending', 'true') 
		ON DUPLICATE KEY UPDATE setting_value = 'true'
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Println("❌ Error setting sync_pending:", err)
		sendTelegramMessage(botToken, chatID, "❌ Error: Gagal set sync flag")
		return
	}

	// Also record timestamp kapan sync di-request
	query2 := `
		INSERT INTO settings (setting_key, setting_value) 
		VALUES ('sync_requested_at', ?) 
		ON DUPLICATE KEY UPDATE setting_value = ?
	`
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err = db.Exec(query2, now, now)
	if err != nil {
		log.Println("⚠️  Warning: Gagal record sync timestamp:", err)
	}

	message := fmt.Sprintf(
		"🔄 SYNC DIPICU\nESP32 akan sync kartu pada loop berikutnya\nWaktu: %s",
		time.Now().Format("15:04:05"),
	)
	sendTelegramMessage(botToken, chatID, message)
	log.Println("✅ /sync command received - sync_pending set to true")
}

// handleStatusCommand: Handle /status command
func handleStatusCommand(db *sql.DB, botToken string, chatID int) {
	// Get device status
	var deviceName, relayStatus, lastHeartbeat string

	db.QueryRow("SELECT setting_value FROM settings WHERE setting_key = 'device_name'").Scan(&deviceName)
	db.QueryRow("SELECT setting_value FROM settings WHERE setting_key = 'relay_status'").Scan(&relayStatus)
	db.QueryRow("SELECT setting_value FROM settings WHERE setting_key = 'device_last_heartbeat'").Scan(&lastHeartbeat)

	// Get scheduled cards count for today
	now := time.Now()
	todayIndex := int(now.Weekday())
	todayName := hariMap[todayIndex]

	var cardCount int
	db.QueryRow(`
		SELECT COUNT(*) FROM users u
		JOIN schedules s ON u.id = s.user_id
		WHERE u.is_active = TRUE AND u.is_admin = FALSE AND s.hari = ?
	`, todayName).Scan(&cardCount)

	// Get access log count for today
	var accessCount int
	db.QueryRow(`
		SELECT COUNT(*) FROM access_logs 
		WHERE DATE(waktu) = CURDATE()
	`).Scan(&accessCount)

	relayStatusStr := "🔒 LOCKED"
	if relayStatus == "1" {
		relayStatusStr = "🔓 UNLOCKED"
	}

	message := fmt.Sprintf(
		"📊 STATUS PINTU\n\n"+
			"Device: %s\n"+
			"Relay: %s\n"+
			"Last Heartbeat: %s\n"+
			"Hari: %s\n"+
			"Kartu Hari Ini: %d\n"+
			"Akses Hari Ini: %d",
		deviceName, relayStatusStr, lastHeartbeat, todayName, cardCount, accessCount,
	)
	sendTelegramMessage(botToken, chatID, message)
	log.Println("✅ /status command executed")
}

// sendTelegramMessage: Kirim pesan ke Telegram
func sendTelegramMessage(botToken string, chatID int, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Println("❌ Error marshaling JSON:", err)
		return
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonPayload)))
	if err != nil {
		log.Println("❌ Error sending Telegram message:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("⚠️  Telegram returned status %d\n", resp.StatusCode)
	}
}

// StartTelegramBot: Inicialisasi Telegram bot pada startup.
// Implementasi minimal karena aplikasi menggunakan webhook handler.
// Memeriksa konfigurasi di tabel settings dan hanya mencatat status.
func StartTelegramBot(db *sql.DB) error {
	var token, enabled string
	_ = db.QueryRow("SELECT setting_value FROM settings WHERE setting_key = 'telegram_token'").Scan(&token)
	_ = db.QueryRow("SELECT setting_value FROM settings WHERE setting_key = 'telegram_enabled'").Scan(&enabled)

	if token == "" {
		log.Println("Telegram token tidak ditemukan di settings; bot tidak di-start")
		return nil
	}
	if strings.ToLower(enabled) != "true" {
		log.Println("Telegram disabled di settings; bot tidak di-start")
		return nil
	}

	pref := telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return fmt.Errorf("gagal buat telebot: %w", err)
	}

	// Register command handlers
	b.Handle("/sync", func(c telebot.Context) error {
		chatID := int(c.Chat().ID)
		go handleSyncCommand(db, token, chatID)
		return c.Send("🔄 Sync dipicu — ESP akan sync pada loop berikutnya")
	})

	b.Handle("/status", func(c telebot.Context) error {
		chatID := int(c.Chat().ID)
		go handleStatusCommand(db, token, chatID)
		return c.Send("⏳ Mengambil status...")
	})

	b.Handle("/database", func(c telebot.Context) error {
		// Query admin users
		adminRows, err := db.Query("SELECT nama, uid FROM users WHERE is_admin = TRUE ORDER BY nama")
		if err != nil {
			return c.Send("❌ Error querying admin users: " + err.Error())
		}
		defer adminRows.Close()

		var adminUsers []struct{ Nama, UID string }
		for adminRows.Next() {
			var nama, uid string
			if err := adminRows.Scan(&nama, &uid); err != nil {
				continue
			}
			adminUsers = append(adminUsers, struct{ Nama, UID string }{nama, uid})
		}

		userRows, err := db.Query("SELECT nama, uid FROM users WHERE is_admin = FALSE ORDER BY nama")
		if err != nil {
			return c.Send("❌ Error querying users: " + err.Error())
		}
		defer userRows.Close()

		var regularUsers []struct{ Nama, UID string }
		for userRows.Next() {
			var nama, uid string
			if err := userRows.Scan(&nama, &uid); err != nil {
				continue
			}
			regularUsers = append(regularUsers, struct{ Nama, UID string }{nama, uid})
		}

		// Build response
		response := "📊 DATABASE USERS\n\n"
		response += "👑 ADMIN (" + strconv.Itoa(len(adminUsers)) + " orang):\n"
		if len(adminUsers) == 0 {
			response += "  (kosong)\n"
		} else {
			for _, u := range adminUsers {
				response += "  • " + u.Nama + " → " + u.UID + "\n"
			}
		}
		response += "\n👤 REGULAR (" + strconv.Itoa(len(regularUsers)) + " orang):\n"
		if len(regularUsers) == 0 {
			response += "  (kosong)\n"
		} else {
			for _, u := range regularUsers {
				response += "  • " + u.Nama + " → " + u.UID + "\n"
			}
		}
		return c.Send(response)
	})

	b.Handle("/help", func(c telebot.Context) error {
		msg := "📖 Daftar Commands:\n\n" +
			"• /setjadwal [hari] [nama1, nama2, ...]\n  Set jadwal akses untuk hari tertentu\n\n" +
			"• /lihatjadwal [hari]\n  Lihat jadwal akses (atau tanpa argumen untuk semua hari)\n\n" +
			"• /database\n  Lihat daftar pengguna dan UID dari database\n\n" +
			"• /status\n  Lihat status dan informasi perangkat ESP\n\n" +
			"• /sync\n  Sync kartu ke ESP32 segera\n"
		return c.Send(msg)
	})

	b.Handle("/setjadwal", func(c telebot.Context) error {
		text := strings.TrimSpace(c.Text())
		parts := strings.Fields(text)
		if len(parts) < 3 {
			return c.Send("❌ Format salah! Gunakan: /setjadwal [hari] [nama1, nama2, ...]")
		}
		hariRaw := parts[1]
		hari := strings.ToUpper(hariRaw[:1]) + strings.ToLower(hariRaw[1:])
		validDays := map[string]bool{"Senin": true, "Selasa": true, "Rabu": true, "Kamis": true, "Jumat": true}
		if !validDays[hari] {
			return c.Send("❌ Hari tidak valid: " + hari)
		}
		namesStr := strings.Join(parts[2:], " ")
		names := strings.Split(namesStr, ",")

		_, err := db.Exec("DELETE FROM schedules WHERE hari = ?", hari)
		if err != nil {
			return c.Send("❌ Error menghapus jadwal lama: " + err.Error())
		}

		inserted := []string{}
		notFound := []string{}
		adminSkipped := []string{}
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" || name == "-" || strings.ToLower(name) == "kosong" {
				continue
			}
			nameUpper := strings.ToUpper(name)
			var userID int
			var isAdmin bool
			err := db.QueryRow("SELECT id, is_admin FROM users WHERE UPPER(nama) = ? AND is_active = TRUE", nameUpper).Scan(&userID, &isAdmin)
			if err != nil {
				notFound = append(notFound, nameUpper)
				continue
			}
			if isAdmin {
				adminSkipped = append(adminSkipped, nameUpper)
				continue
			}
			_, err = db.Exec("INSERT INTO schedules (user_id, hari) VALUES (?, ?)", userID, hari)
			if err == nil {
				inserted = append(inserted, nameUpper)
			}
		}

		resp := "✅ Jadwal " + hari + " berhasil diupdate!\n\n"
		resp += "📋 Terjadwal (" + strconv.Itoa(len(inserted)) + " orang):\n"
		if len(inserted) > 0 {
			for _, n := range inserted {
				resp += "  • " + n + "\n"
			}
		} else {
			resp += "  (kosong)\n"
		}
		if len(adminSkipped) > 0 {
			resp += "\n⚠️ Tidak perlu dijadwal (admin):\n"
			for _, n := range adminSkipped {
				resp += "  • " + n + "\n"
			}
		}
		if len(notFound) > 0 {
			resp += "\n❌ Tidak ditemukan: \n"
			for _, n := range notFound {
				resp += "  • " + n + "\n"
			}
		}
		return c.Send(resp)
	})

	b.Handle("/lihatjadwal", func(c telebot.Context) error {
		text := strings.TrimSpace(c.Text())
		parts := strings.Fields(text)
		hariFilter := ""
		if len(parts) >= 2 {
			hariRaw := parts[1]
			hariFilter = strings.ToUpper(hariRaw[:1]) + strings.ToLower(hariRaw[1:])
		}
		validDays := []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat"}
		daysToShow := validDays
		if hariFilter != "" {
			found := false
			for _, d := range validDays {
				if d == hariFilter {
					found = true
					break
				}
			}
			if !found {
				return c.Send("❌ Hari tidak valid. Gunakan: Senin, Selasa, Rabu, Kamis, Jumat")
			}
			daysToShow = []string{hariFilter}
		}
		response := "📅 Jadwal Akses Pintu\n\n"
		for _, hari := range daysToShow {
			rows, err := db.Query(`SELECT u.nama FROM users u JOIN schedules s ON u.id = s.user_id WHERE s.hari = ? AND u.is_active = TRUE ORDER BY u.nama`, hari)
			if err != nil {
				continue
			}
			var names []string
			for rows.Next() {
				var nama string
				if rows.Scan(&nama) == nil {
					names = append(names, nama)
				}
			}
			rows.Close()
			response += "📌 " + hari + " (" + strconv.Itoa(len(names)) + " orang):\n"
			if len(names) == 0 {
				response += "  (kosong)\n"
			} else {
				for _, n := range names {
					response += "  • " + n + "\n"
				}
			}
			response += "\n"
		}
		return c.Send(response)
	})

	b.Handle("/device", func(c telebot.Context) error {
		// Read device settings
		deviceSettings := map[string]string{}
		keys := []string{"device_type", "device_name", "device_started_at", "device_last_heartbeat", "relay_status", "door_name"}
		for _, k := range keys {
			var v string
			_ = db.QueryRow("SELECT setting_value FROM settings WHERE setting_key = ?", k).Scan(&v)
			deviceSettings[k] = v
		}
		if deviceSettings["device_type"] == "" {
			deviceSettings["device_type"] = "ESP32"
		}
		if deviceSettings["device_name"] == "" {
			deviceSettings["device_name"] = "RFID Door Lock System"
		}
		if deviceSettings["door_name"] == "" {
			deviceSettings["door_name"] = "Main Door"
		}

		uptime := "Unknown"
		if deviceSettings["device_started_at"] != "" {
			if st, err := time.Parse("2006-01-02 15:04:05", deviceSettings["device_started_at"]); err == nil {
				d := time.Since(st)
				h := int(d.Hours())
				m := int(d.Minutes()) % 60
				uptime = fmt.Sprintf("%dh %dm", h, m)
			}
		}
		deviceStatus := "🔴 OFFLINE"
		lastHB := "Tidak ada data"
		if deviceSettings["device_last_heartbeat"] != "" {
			if t, err := time.Parse("2006-01-02 15:04:05", deviceSettings["device_last_heartbeat"]); err == nil {
				lastHB = t.Format("02-01-2006 15:04:05")
				if time.Since(t) < 5*time.Minute {
					deviceStatus = "🟢 ONLINE"
				} else if time.Since(t) < 1*time.Hour {
					deviceStatus = "🟡 IDLE"
				}
			}
		}
		relay := "Unknown"
		if deviceSettings["relay_status"] == "1" {
			relay = "🔓 Terbuka"
		} else if deviceSettings["relay_status"] == "0" {
			relay = "🔒 Tertutup"
		}

		today := time.Now().Format("2006-01-02")
		var totalAccessToday, grantedToday, deniedToday, totalUsers int
		_ = db.QueryRow("SELECT COUNT(*) FROM access_logs WHERE DATE(waktu) = ?", today).Scan(&totalAccessToday)
		_ = db.QueryRow("SELECT COUNT(*) FROM access_logs WHERE DATE(waktu) = ? AND status = 'GRANTED'", today).Scan(&grantedToday)
		_ = db.QueryRow("SELECT COUNT(*) FROM access_logs WHERE DATE(waktu) = ? AND status = 'DENIED'", today).Scan(&deniedToday)
		_ = db.QueryRow("SELECT COUNT(*) FROM users WHERE is_active = TRUE").Scan(&totalUsers)

		resp := "🖥️ STATUS PERANGKAT\n\n"
		resp += fmt.Sprintf("📱 Jenis: %s\n", deviceSettings["device_type"])
		resp += fmt.Sprintf("💻 Nama: %s\n", deviceSettings["device_name"])
		resp += fmt.Sprintf("🚪 Pintu: %s\n\n", deviceSettings["door_name"])
		resp += fmt.Sprintf("Status: %s\n⏱️ Uptime: %s\n🔔 Terakhir Aktif: %s\n🔐 Status Pintu: %s\n\n", deviceStatus, uptime, lastHB, relay)
		resp += fmt.Sprintf("📊 Statistik Hari Ini (%s):\n  ✅ Granted: %d\n  ❌ Denied: %d\n  📈 Total: %d\n\n", today, grantedToday, deniedToday, totalAccessToday)
		resp += fmt.Sprintf("👥 Total User Aktif: %d\n⚙️ Server Time: %s", totalUsers, time.Now().Format("02-01-2006 15:04:05"))
		return c.Send(resp)
	})

	// Start polling in background
	go b.Start()
	log.Println("✅ Telegram bot polling started")
	return nil
}

// ===== UNTUK ESP32 =====

// GetSyncStatusHandler: Endpoint untuk ESP32 check apakah ada pending sync
// Endpoint: GET /api/sync-status
func GetSyncStatusHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	var syncPending string
	err := db.QueryRow("SELECT setting_value FROM settings WHERE setting_key = 'sync_pending'").Scan(&syncPending)

	// Default false kalau tidak ada record
	if err != nil || syncPending == "" {
		syncPending = "false"
	}

	shouldSync := syncPending == "true"

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"should_sync": shouldSync,
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
	})
}

// ConfirmSyncHandler: ESP32 confirm sync setelah selesai
// Endpoint: POST /api/confirm-sync
func ConfirmSyncHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// Reset sync_pending flag ke false
	query := `
		INSERT INTO settings (setting_key, setting_value) 
		VALUES ('sync_pending', 'false') 
		ON DUPLICATE KEY UPDATE setting_value = 'false'
	`
	_, err := db.Exec(query)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "Database error"})
		return
	}

	// Record sync completed time
	query2 := `
		INSERT INTO settings (setting_key, setting_value) 
		VALUES ('sync_completed_at', ?) 
		ON DUPLICATE KEY UPDATE setting_value = ?
	`
	now := time.Now().Format("2006-01-02 15:04:05")
	_, err = db.Exec(query2, now, now)

	log.Println("✅ /api/confirm-sync - sync completed")

	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "Sync confirmed",
	})
}
