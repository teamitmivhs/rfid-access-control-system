package handlers

import (
	"database/sql"
	"door-lock-system/models"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Hari kerja: Senin (1) - Jumat (5)
var hariMap = map[int]string{
	0: "Minggu",
	1: "Senin",
	2: "Selasa",
	3: "Rabu",
	4: "Kamis",
	5: "Jumat",
	6: "Sabtu",
}

// JSON response helper
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// VerifyAccessHandler: Handler untuk verifikasi akses kartu RFID
func VerifyAccessHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	var req models.AccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request: uid required"})
		return
	}

	// Cari user berdasarkan UID
	user, err := getUserByUID(db, req.UID)
	if err != nil {
		logAccess(db, nil, req.UID, "UNKNOWN", "DENIED")
		jsonResponse(w, http.StatusOK, models.AccessResponse{
			Allowed: false,
			Reason:  "User not found",
		})
		return
	}

	// Cek jika user aktif
	if !user.IsActive {
		logAccess(db, &user.ID, req.UID, user.Nama, "DENIED")
		jsonResponse(w, http.StatusOK, models.AccessResponse{
			Allowed: false,
			Name:    user.Nama,
			Reason:  "User deactivated",
		})
		return
	}

	// Cek jika user adalah admin - bypass jadwal
	if user.IsAdmin {
		logAccess(db, &user.ID, req.UID, user.Nama, "GRANTED")
		// send telegram notification (non-blocking)
		if cfg, err := GetTelegramConfig(db); err == nil && cfg.Enabled {
			msg := fmt.Sprintf("✅ ACCESS GRANTED\nNama: %s\nKartu: %s\nTipe: ADMIN\n%s", user.Nama, req.UID, time.Now().Format("02-01-2006 15:04:05"))
			go KirimNotifikasi(cfg, msg)
		}
		jsonResponse(w, http.StatusOK, models.AccessResponse{
			Allowed: true,
			Name:    user.Nama,
			Message: "Access Granted (Admin)",
		})
		return
	}

	// Ambil hari saat ini
	now := time.Now()
	todayIndex := int(now.Weekday())
	todayName := hariMap[todayIndex]

	// Jika hari Sabtu/Minggu, akses ditolak
	if todayIndex == 0 || todayIndex == 6 {
		logAccess(db, &user.ID, req.UID, user.Nama, "SCHEDULE_DENIED")
		jsonResponse(w, http.StatusOK, models.AccessResponse{
			Allowed: false,
			Name:    user.Nama,
			Reason:  "Access only allowed Senin-Jumat",
		})
		return
	}

	// Cek schedule
	isScheduled, err := isUserScheduledForDay(db, user.ID, todayName)
	if err != nil {
		log.Println("Error checking schedule:", err)
		jsonResponse(w, http.StatusInternalServerError, models.ErrorResponse{Error: "Database error"})
		return
	}

	if isScheduled {
		logAccess(db, &user.ID, req.UID, user.Nama, "GRANTED")
		if cfg, err := GetTelegramConfig(db); err == nil && cfg.Enabled {
			msg := fmt.Sprintf("✅ ACCESS GRANTED\nNama: %s\nKartu: %s\nTipe: SCHEDULED\n%s", user.Nama, req.UID, time.Now().Format("02-01-2006 15:04:05"))
			go KirimNotifikasi(cfg, msg)
		}
		jsonResponse(w, http.StatusOK, models.AccessResponse{
			Allowed: true,
			Name:    user.Nama,
			Message: "Access Granted",
		})
	} else {
		logAccess(db, &user.ID, req.UID, user.Nama, "SCHEDULE_DENIED")
		if cfg, err := GetTelegramConfig(db); err == nil && cfg.Enabled {
			msg := fmt.Sprintf("❌ ACCESS DENIED\nKartu: %s\nNama: %s\nAlasan: Not authorized\n%s", req.UID, user.Nama, time.Now().Format("02-01-2006 15:04:05"))
			go KirimNotifikasi(cfg, msg)
		}
		jsonResponse(w, http.StatusOK, models.AccessResponse{
			Allowed: false,
			Name:    user.Nama,
			Reason:  "Not scheduled for today (" + todayName + ")",
		})
	}
}

// getUserByUID: Query user dari database berdasarkan UID
func getUserByUID(db *sql.DB, uid string) (*models.User, error) {
	user := &models.User{}
	err := db.QueryRow(
		"SELECT id, uid, nama, is_active, is_admin, created_at, updated_at FROM users WHERE uid = ?",
		uid,
	).Scan(&user.ID, &user.UID, &user.Nama, &user.IsActive, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return user, nil
}

// isUserScheduledForDay: Cek apakah user dijadwalkan untuk hari tertentu
func isUserScheduledForDay(db *sql.DB, userID int, hari string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM schedules WHERE user_id = ? AND hari = ?",
		userID, hari,
	).Scan(&count)

	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// logAccess: Insert access log ke database
func logAccess(db *sql.DB, userID *int, uid, nama, status string) {
	query := "INSERT INTO access_logs (user_id, uid, nama, status, waktu) VALUES (?, ?, ?, ?, ?)"
	_, err := db.Exec(query, userID, uid, nama, status, time.Now())
	if err != nil {
		log.Println("Error logging access:", err)
	}
}

// CardForToday: Struct untuk response kartu hari ini
type CardForToday struct {
	UID  string `json:"uid"`
	Nama string `json:"nama"`
	Type string `json:"type"` // "admin" atau "scheduled"
}

// GetCardsForTodayHandler: Get all authorized cards for today (admin + scheduled)
// Endpoint lama - /api/cards/today - masih ada untuk kompatibilitas
func GetCardsForTodayHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	todayIndex := int(now.Weekday())
	todayName := hariMap[todayIndex]

	cards := []CardForToday{}

	// 1. Query semua admin cards
	adminQuery := "SELECT uid, nama FROM users WHERE is_active = TRUE AND is_admin = TRUE"
	rows, err := db.Query(adminQuery)
	if err != nil {
		log.Println("Error querying admin cards:", err)
		jsonResponse(w, http.StatusInternalServerError, models.ErrorResponse{Error: "Database error"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var uid, nama string
		if err := rows.Scan(&uid, &nama); err != nil {
			log.Println("Error scanning admin card:", err)
			continue
		}
		cards = append(cards, CardForToday{UID: uid, Nama: nama, Type: "admin"})
	}

	// 2. Query scheduled cards untuk hari ini
	scheduledQuery := `
		SELECT u.uid, u.nama FROM users u
		JOIN schedules s ON u.id = s.user_id
		WHERE u.is_active = TRUE AND u.is_admin = FALSE AND s.hari = ?
	`
	rows2, err := db.Query(scheduledQuery, todayName)
	if err != nil {
		log.Println("Error querying scheduled cards:", err)
		jsonResponse(w, http.StatusInternalServerError, models.ErrorResponse{Error: "Database error"})
		return
	}
	defer rows2.Close()

	for rows2.Next() {
		var uid, nama string
		if err := rows2.Scan(&uid, &nama); err != nil {
			log.Println("Error scanning scheduled card:", err)
			continue
		}
		cards = append(cards, CardForToday{UID: uid, Nama: nama, Type: "scheduled"})
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"hari":  todayName,
		"cards": cards,
		"count": len(cards),
	})
}

// GetScheduledCardsForTodayHandler: Endpoint baru - /api/cards/scheduled-today
// BUG FIX: Hanya mengembalikan scheduled (non-admin) cards untuk di-cache ESP32.
// Admin cards sudah hardcode di ESP32 jadi tidak perlu dikirim lagi lewat endpoint ini,
// sehingga tidak ada duplikasi pengecekan di sisi ESP32.
func GetScheduledCardsForTodayHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	todayIndex := int(now.Weekday())
	todayName := hariMap[todayIndex]

	cards := []CardForToday{}

	// Hanya query scheduled (non-admin) cards untuk hari ini
	scheduledQuery := `
		SELECT u.uid, u.nama FROM users u
		JOIN schedules s ON u.id = s.user_id
		WHERE u.is_active = TRUE AND u.is_admin = FALSE AND s.hari = ?
	`
	rows, err := db.Query(scheduledQuery, todayName)
	if err != nil {
		log.Println("Error querying scheduled cards:", err)
		jsonResponse(w, http.StatusInternalServerError, models.ErrorResponse{Error: "Database error"})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var uid, nama string
		if err := rows.Scan(&uid, &nama); err != nil {
			log.Println("Error scanning scheduled card:", err)
			continue
		}
		cards = append(cards, CardForToday{UID: uid, Nama: nama, Type: "scheduled"})
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"hari":  todayName,
		"cards": cards,
		"count": len(cards),
	})
}

// DeviceHeartbeatRequest: Struktur untuk heartbeat dari ESP32
type DeviceHeartbeatRequest struct {
	DeviceType   string `json:"device_type"`   // e.g., "ESP32"
	DeviceName   string `json:"device_name"`   // e.g., "RFID Door Lock"
	RelayStatus  string `json:"relay_status"`  // "0" (closed) atau "1" (open)
	WiFiStrength int    `json:"wifi_strength"` // RSSI value
	FreeMemory   int    `json:"free_memory"`   // Available heap memory
	Uptime       int64  `json:"uptime"`        // Uptime in seconds
}

// DeviceHeartbeatHandler: Handle heartbeat/status update dari ESP32
// Endpoint: POST /api/device/heartbeat
// Fungsi ini memperbarui status perangkat di tabel settings
func DeviceHeartbeatHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	var req DeviceHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}
	if req.DeviceName == "" || (req.RelayStatus != "0" && req.RelayStatus != "1") ||
		req.Uptime < 0 || req.Uptime > 10*365*24*60*60 {
		jsonResponse(w, http.StatusBadRequest, models.ErrorResponse{Error: "Invalid heartbeat data"})
		return
	}

	// Update device settings di database
	settingsToUpdate := map[string]string{
		"device_type":           req.DeviceType,
		"device_name":           req.DeviceName,
		"relay_status":          req.RelayStatus,
		"device_last_heartbeat": time.Now().Format("2006-01-02 15:04:05"),
		"device_started_at":     time.Now().Add(-time.Duration(req.Uptime) * time.Second).Format("2006-01-02 15:04:05"),
		"device_uptime":         strconv.FormatInt(req.Uptime, 10),
		"device_wifi_strength":  strconv.Itoa(req.WiFiStrength),
		"device_free_memory":    strconv.Itoa(req.FreeMemory),
	}

	// Update semua settings
	for key, value := range settingsToUpdate {
		_, err := db.Exec(
			"INSERT INTO settings (setting_key, setting_value) VALUES (?, ?) ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value)",
			key, value,
		)
		if err != nil {
			log.Println("Error updating setting "+key+":", err)
			jsonResponse(w, http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to save heartbeat"})
			return
		}
	}

	// Log successful heartbeat
	log.Printf("✅ Device heartbeat received: %s (WiFi: %d dBm, Memory: %d bytes)", req.DeviceName, req.WiFiStrength, req.FreeMemory)

	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "Heartbeat received",
	})
}

// RegisterReportRequest: dari ESP ketika mode pendaftaran aktif dan kartu ditap
type RegisterReportRequest struct {
	UID  string `json:"uid"`
	Mode string `json:"mode"` // "normal" atau "admin"
}

// RegisterReportHandler: ESP POST /api/device/register-report
// Server mencari pending registration tanpa UID dan mengisi UID, lalu meminta nama dari Telegram user
func RegisterReportHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	var req RegisterReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	mode := req.Mode
	if mode != "admin" {
		mode = "normal"
	}

	log.Printf("RegisterReportHandler: received UID=%s mode=%s", req.UID, mode)

	// Find oldest pending registration without UID for this mode
	var id int
	var telegramUserID, chatID string
	query := `SELECT id, telegram_user_id, chat_id FROM registration_pending WHERE uid IS NULL AND mode = ? ORDER BY created_at LIMIT 1`
	err := db.QueryRow(query, mode).Scan(&id, &telegramUserID, &chatID)
	if err != nil {
		log.Println("RegisterReportHandler: no pending registration found for mode", mode)
		jsonResponse(w, http.StatusOK, map[string]string{"status": "no_pending", "message": "No pending registration"})
		return
	}

	// Update pending registration with UID and mark awaiting_name = true
	res, err := db.Exec("UPDATE registration_pending SET uid = ?, awaiting_name = TRUE, updated_at = NOW() WHERE id = ?", req.UID, id)
	if err != nil {
		log.Println("RegisterReportHandler: failed update pending registration:", err)
		jsonResponse(w, http.StatusInternalServerError, models.ErrorResponse{Error: "Database error"})
		return
	}
	affected, _ := res.RowsAffected()
	log.Printf("RegisterReportHandler: updated pending id=%d, rowsAffected=%d", id, affected)

	// Notify the Telegram user (direct message) to enter the name
	cfg, err := GetTelegramConfig(db)
	if err == nil && cfg.Enabled {
		// Always notify the configured group first so operators see the UID immediately
		groupMsg := "🔔 UID terdeteksi: " + req.UID + "\nOperator: mohon balas dengan NAMA pemilik UID (tanpa '/')."
		_ = KirimNotifikasi(cfg, groupMsg)

		// chatID stored as string; try parse to int for DM
		if chatInt, parseErr := strconv.Atoi(chatID); parseErr == nil {
			dmMsg := "✅ UID terdeteksi: " + req.UID + "\nSilakan balas dengan NAMA pemilik UID (tanpa '/')."
			if err := sendTelegramMessage(cfg.Token, chatInt, dmMsg); err != nil {
				log.Println("RegisterReportHandler: failed to send DM to user:", err)
				// DM failed, group already notified
			} else {
				log.Printf("RegisterReportHandler: DM sent to telegram_user_id=%s chat_id=%s", telegramUserID, chatID)
			}
		} else {
			// chat id parsing failed; group already notified
			log.Println("RegisterReportHandler: chat_id parse error, group notified")
		}
	} else if err != nil {
		log.Println("RegisterReportHandler: GetTelegramConfig error:", err)
	}

	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok", "message": "UID recorded"})
}

// GetRegistrationModeHandler: return the next pending registration mode for ESP to poll
// Endpoint: GET /api/registration/pending-mode
func GetRegistrationModeHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// Find oldest pending registration without UID
	var mode string
	var chatID string
	query := `SELECT mode, chat_id FROM registration_pending WHERE uid IS NULL ORDER BY created_at LIMIT 1`
	err := db.QueryRow(query).Scan(&mode, &chatID)
	if err != nil {
		// No pending registration
		jsonResponse(w, http.StatusOK, map[string]string{"mode": "", "chat_id": ""})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"mode": mode, "chat_id": chatID})
}

// getPendingByTelegramUser: helper to fetch pending registration by telegram user id
func getPendingByTelegramUser(db *sql.DB, telegramUserID string) (int, string, string, bool, error) {
	var id int
	var uid sql.NullString
	var mode string
	var awaiting bool
	err := db.QueryRow("SELECT id, uid, mode, awaiting_name FROM registration_pending WHERE telegram_user_id = ? LIMIT 1", telegramUserID).Scan(&id, &uid, &mode, &awaiting)
	if err != nil {
		return 0, "", "", false, err
	}
	u := ""
	if uid.Valid {
		u = uid.String
	}
	return id, u, mode, awaiting, nil
}

// completePendingRegistration: persist new user and mark pending as completed
func completePendingRegistration(db *sql.DB, pendingID int, uid, name string, isAdmin bool) error {
	// Insert into users (ignore if exists)
	_, err := db.Exec("INSERT INTO users (uid, nama, is_admin, is_active) VALUES (?, ?, ?, TRUE) ON DUPLICATE KEY UPDATE nama = VALUES(nama), is_admin = VALUES(is_admin), is_active = TRUE", uid, name, isAdmin)
	if err != nil {
		return err
	}
	// Mark pending registration removed
	_, err = db.Exec("DELETE FROM registration_pending WHERE id = ?", pendingID)
	return err
}
