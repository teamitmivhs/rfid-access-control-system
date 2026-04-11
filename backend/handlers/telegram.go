package handlers

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

func SetJadwalHandler(db *sql.DB, c telebot.Context) error {
	// Parse: /setjadwal senin ALVARO, FIKRI, GANI
	text := c.Text()
	parts := strings.Fields(text)

	if len(parts) < 3 {
		return c.Send("❌ Format salah!\nGunakan: /setjadwal [hari] [nama1, nama2, ...]\n\nContoh: /setjadwal senin ALVARO, FIKRI, GANI")
	}

	hari := strings.ToUpper(parts[1][:1]) + strings.ToLower(parts[1][1:]) // Capitalize first letter
	namesStr := strings.Join(parts[2:], " ")
	names := strings.Split(namesStr, ",")

	// Validasi hari
	validDays := []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	valid := false
	for _, day := range validDays {
		if hari == day {
			valid = true
			break
		}
	}
	if !valid {
		return c.Send("❌ Hari tidak valid!\nGunakan: Senin, Selasa, Rabu, Kamis, Jumat, Sabtu, Minggu")
	}

	// Hapus schedule lama untuk hari itu
	_, err := db.Exec("DELETE FROM schedules WHERE hari = ?", hari)
	if err != nil {
		return c.Send("❌ Error menghapus jadwal lama: " + err.Error())
	}

	insertedCount := 0
	notFoundCount := 0

	// Insert jadwal baru
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || name == "-" || strings.ToLower(name) == "kosong" {
			continue
		}

		nameLower := strings.ToUpper(name)

		// Cari user_id berdasarkan nama
		var userID int
		err := db.QueryRow("SELECT id FROM users WHERE UPPER(nama) = ?", nameLower).Scan(&userID)
		if err != nil {
			notFoundCount++
			continue
		}

		_, err = db.Exec("INSERT INTO schedules (user_id, hari) VALUES (?, ?)", userID, hari)
		if err == nil {
			insertedCount++
		}
	}

	// Build response message
	response := "✓ Jadwal " + hari + " berhasil diupdate!\n\n"
	response += "📊 Summary:\n"
	response += "✅ Ditambahkan: " + strconv.Itoa(insertedCount) + " orang\n"

	if notFoundCount > 0 {
		response += "⚠️  Tidak ditemukan: " + strconv.Itoa(notFoundCount) + " orang"
	}

	return c.Send(response)
}

func StartTelegramBot(db *sql.DB, token string) error {
	pref := telebot.Settings{
		Token:  token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return err
	}

	// Register command handler
	b.Handle("/setjadwal", func(c telebot.Context) error {
		return SetJadwalHandler(db, c)
	})

	// Simple ping handler
	b.Handle("/start", func(c telebot.Context) error {
		return c.Send("🤖 Bot jadwal pintu aktif!\n\nCommand: /setjadwal [hari] [nama1, nama2, ...]")
	})

	b.Start()
	return nil
}
