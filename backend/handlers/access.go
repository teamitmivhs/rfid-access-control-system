package handlers

import (
	"database/sql"
	"door-lock-system/models"
	"encoding/json"
	"log"
	"net/http"
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

// VerifyAccessHandler: Handler untuk verifikasi akses kartu RFID (standard library version)
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

	// Cek apakah user dijadwalkan hari ini (Senin-Jumat saja)
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
		jsonResponse(w, http.StatusOK, models.AccessResponse{
			Allowed: true,
			Name:    user.Nama,
			Message: "Access Granted",
		})
	} else {
		logAccess(db, &user.ID, req.UID, user.Nama, "SCHEDULE_DENIED")
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
