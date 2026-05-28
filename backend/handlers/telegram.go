package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
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
