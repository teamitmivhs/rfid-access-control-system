package models

import "time"

type Schedule struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	Hari      string    `json:"hari"`
	CreatedAt time.Time `json:"created_at"`
}

// ScheduleRequest: Update jadwal dengan hanya 5 hari kerja (Senin-Jumat)
type ScheduleRequest struct {
	Senin  []string `json:"Senin"`
	Selasa []string `json:"Selasa"`
	Rabu   []string `json:"Rabu"`
	Kamis  []string `json:"Kamis"`
	Jumat  []string `json:"Jumat"`
}

type ScheduleResponse struct {
	Message string `json:"message"`
	Updated int    `json:"updated"`
}
