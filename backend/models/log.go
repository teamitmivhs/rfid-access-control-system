package models

import "time"

type AccessLog struct {
	ID     int       `json:"id"`
	UserID *int      `json:"user_id"`
	UID    string    `json:"uid"`
	Nama   string    `json:"nama"`
	Status string    `json:"status"`
	Waktu  time.Time `json:"waktu"`
}
