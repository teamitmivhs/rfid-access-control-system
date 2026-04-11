package handlers

import (
	"database/sql"
	"door-lock-system/models"
	"encoding/json"
	"log"
	"net/http"
)

// GetScheduleHandler: Handler untuk get jadwal akses pengguna
func GetScheduleHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		jsonResponse(w, http.StatusBadRequest, models.ErrorResponse{Error: "user_id parameter required"})
		return
	}

	rows, err := db.Query(
		"SELECT id, user_id, hari FROM schedules WHERE user_id = ? ORDER BY hari",
		userID,
	)
	if err != nil {
		log.Println("Error querying schedules:", err)
		jsonResponse(w, http.StatusInternalServerError, models.ErrorResponse{Error: "Database error"})
		return
	}
	defer rows.Close()

	var schedules []models.Schedule
	for rows.Next() {
		var sched models.Schedule
		if err := rows.Scan(&sched.ID, &sched.UserID, &sched.Hari); err != nil {
			log.Println("Error scanning schedule:", err)
			continue
		}
		schedules = append(schedules, sched)
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{"schedules": schedules})
}

// UpdateScheduleHandler: Update jadwal akses untuk satu pengguna
// Request: {"user_id": 1, "days": ["Senin", "Selasa", "Rabu", "Kamis", "Jumat"]}
func UpdateScheduleHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID int      `json:"user_id"`
		Days   []string `json:"days"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request"})
		return
	}

	// Validasi hari hanya Senin-Jumat
	validDays := map[string]bool{
		"Senin":  true,
		"Selasa": true,
		"Rabu":   true,
		"Kamis":  true,
		"Jumat":  true,
	}

	for _, day := range req.Days {
		if !validDays[day] {
			jsonResponse(w, http.StatusBadRequest, models.ErrorResponse{
				Error: "Invalid day: " + day + " (only Senin-Jumat allowed)",
			})
			return
		}
	}

	// Hapus jadwal lama untuk user ini
	_, err := db.Exec("DELETE FROM schedules WHERE user_id = ?", req.UserID)
	if err != nil {
		log.Println("Error deleting old schedules:", err)
		jsonResponse(w, http.StatusInternalServerError, models.ErrorResponse{Error: "Database error"})
		return
	}

	// Insert jadwal baru
	insertCount := 0
	for _, day := range req.Days {
		result, err := db.Exec(
			"INSERT INTO schedules (user_id, hari) VALUES (?, ?)",
			req.UserID, day,
		)
		if err != nil {
			log.Println("Error inserting schedule:", err)
			continue
		}

		rowsAffected, _ := result.RowsAffected()
		insertCount += int(rowsAffected)
	}

	jsonResponse(w, http.StatusOK, models.ScheduleResponse{
		Message: "Schedule updated successfully",
		Updated: insertCount,
	})
}
