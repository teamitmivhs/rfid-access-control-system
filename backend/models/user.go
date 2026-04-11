package models

import "time"

type User struct {
	ID        int       `json:"id"`
	UID       string    `json:"uid"`
	Nama      string    `json:"nama"`
	IsActive  bool      `json:"is_active"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AccessRequest struct {
	UID       string `json:"uid" binding:"required"`
	Timestamp string `json:"timestamp"`
}

type AccessResponse struct {
	Allowed bool   `json:"allowed"`
	Name    string `json:"name"`
	Message string `json:"message"`
	Reason  string `json:"reason,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
