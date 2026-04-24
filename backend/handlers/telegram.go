package handlers

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

// TelegramConfig: Konfigurasi bot yang dibaca dari tabel settings di database
type TelegramConfig struct {
	Token   string
	ChatID  string
	Enabled bool
}

// GetTelegramConfig: Baca token dan chat_id dari tabel settings
// Ini adalah fungsi kunci yang menghubungkan server ke bot Telegram.
// Token dan chat_id disimpan di database (bukan hardcode), sehingga
// bisa diubah tanpa perlu recompile server.
func GetTelegramConfig(db *sql.DB) (*TelegramConfig, error) {
	cfg := &TelegramConfig{}

	rows, err := db.Query(
		"SELECT setting_key, setting_value FROM settings WHERE setting_key IN ('telegram_token', 'telegram_chat_id', 'telegram_enabled')",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query telegram settings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		switch key {
		case "telegram_token":
			cfg.Token = value
		case "telegram_chat_id":
			cfg.ChatID = value
		case "telegram_enabled":
			cfg.Enabled = value == "true"
		}
	}

	if cfg.Token == "" {
		return nil, fmt.Errorf("telegram_token belum diisi di tabel settings")
	}
	if cfg.ChatID == "" {
		return nil, fmt.Errorf("telegram_chat_id belum diisi di tabel settings")
	}

	return cfg, nil
}

// KirimNotifikasi: Kirim pesan ke grup Telegram dari sisi server.
// Dipakai untuk notifikasi akses pintu, perubahan jadwal, dll.
// Menggunakan HTTP langsung (bukan telebot) agar bisa dipanggil
// dari handler mana pun tanpa perlu instance bot.
func KirimNotifikasi(cfg *TelegramConfig, pesan string) error {
	if !cfg.Enabled {
		return nil
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.Token)

	resp, err := http.PostForm(apiURL, url.Values{
		"chat_id": {cfg.ChatID},
		"text":    {pesan},
	})
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// SetJadwalHandler: Handle command /setjadwal dari bot Telegram
// Format: /setjadwal [hari] [nama1, nama2, ...]
// Contoh: /setjadwal senin ALVARO, AKBAR, JEKI
func SetJadwalHandler(db *sql.DB, c telebot.Context) error {
	text := c.Text()
	parts := strings.Fields(text)

	if len(parts) < 3 {
		return c.Send(
			"❌ Format salah!\n" +
				"Gunakan: /setjadwal [hari] [nama1, nama2, ...]\n\n" +
				"Contoh: /setjadwal senin ALVARO, AKBAR, JEKI\n\n" +
				"Hari yang valid: Senin, Selasa, Rabu, Kamis, Jumat",
		)
	}

	hariRaw := parts[1]
	hari := strings.ToUpper(hariRaw[:1]) + strings.ToLower(hariRaw[1:])

	// Hanya Senin-Jumat (sesuai ENUM di schema.sql)
	validDays := map[string]bool{
		"Senin": true, "Selasa": true, "Rabu": true, "Kamis": true, "Jumat": true,
	}
	if !validDays[hari] {
		return c.Send("❌ Hari tidak valid: " + hari + "\nGunakan: Senin, Selasa, Rabu, Kamis, atau Jumat")
	}

	namesStr := strings.Join(parts[2:], " ")
	names := strings.Split(namesStr, ",")

	_, err := db.Exec("DELETE FROM schedules WHERE hari = ?", hari)
	if err != nil {
		return c.Send("❌ Error menghapus jadwal lama: " + err.Error())
	}

	insertedNames := []string{}
	notFoundNames := []string{}

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || name == "-" || strings.ToLower(name) == "kosong" {
			continue
		}
		nameUpper := strings.ToUpper(name)

		var userID int
		err := db.QueryRow(
			"SELECT id FROM users WHERE UPPER(nama) = ? AND is_active = TRUE AND is_admin = FALSE",
			nameUpper,
		).Scan(&userID)
		if err != nil {
			notFoundNames = append(notFoundNames, nameUpper)
			continue
		}

		_, err = db.Exec("INSERT INTO schedules (user_id, hari) VALUES (?, ?)", userID, hari)
		if err == nil {
			insertedNames = append(insertedNames, nameUpper)
		}
	}

	// Build response
	response := "✅ Jadwal " + hari + " berhasil diupdate!\n\n"
	response += "📋 Terjadwal (" + strconv.Itoa(len(insertedNames)) + " orang):\n"
	if len(insertedNames) > 0 {
		for _, n := range insertedNames {
			response += "  • " + n + "\n"
		}
	} else {
		response += "  (kosong)\n"
	}
	if len(notFoundNames) > 0 {
		response += "\n⚠️  Tidak ditemukan di database (" + strconv.Itoa(len(notFoundNames)) + " orang):\n"
		for _, n := range notFoundNames {
			response += "  • " + n + "\n"
		}
		response += "\nPastikan nama sesuai yang ada di database."
	}

	return c.Send(response)
}

// LihatJadwalHandler: Handle command /lihatjadwal
// Format: /lihatjadwal [hari] atau /lihatjadwal (semua hari)
func LihatJadwalHandler(db *sql.DB, c telebot.Context) error {
	text := c.Text()
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
		rows, err := db.Query(`
			SELECT u.nama FROM users u
			JOIN schedules s ON u.id = s.user_id
			WHERE s.hari = ? AND u.is_active = TRUE
			ORDER BY u.nama
		`, hari)
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
}

// StartTelegramBot: Jalankan bot Telegram dengan token dari database.
// Fungsi ini dipanggil saat server startup (di main.go).
// Bot berjalan di goroutine terpisah agar tidak memblokir HTTP server.
func StartTelegramBot(db *sql.DB) error {
	// Baca config dari database — sumber tunggal kebenaran untuk token & chat_id
	cfg, err := GetTelegramConfig(db)
	if err != nil {
		return fmt.Errorf("tidak bisa baca telegram config: %w", err)
	}

	if !cfg.Enabled {
		log.Println("Telegram bot disabled di settings")
		return nil
	}

	pref := telebot.Settings{
		Token:  cfg.Token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := telebot.NewBot(pref)
	if err != nil {
		return fmt.Errorf("gagal buat bot: %w", err)
	}

	b.Handle("/setjadwal", func(c telebot.Context) error {
		return SetJadwalHandler(db, c)
	})

	b.Handle("/lihatjadwal", func(c telebot.Context) error {
		return LihatJadwalHandler(db, c)
	})

	b.Handle("/start", func(c telebot.Context) error {
		return c.Send(
			"🤖 Bot Jadwal Pintu Aktif!\n\n" +
				"Commands:\n" +
				"• /setjadwal [hari] [nama1, nama2, ...]\n" +
				"  Contoh: /setjadwal senin ALVARO, AKBAR\n\n" +
				"• /lihatjadwal [hari]\n" +
				"  Contoh: /lihatjadwal senin\n" +
				"  Atau /lihatjadwal untuk semua hari\n\n" +
				"• /help - Lihat daftar lengkap commands\n\n" +
				"• /database - Lihat daftar pengguna & UID",
		)
	})

	b.Handle("/help", func(c telebot.Context) error {
		return c.Send(
			"📖 Daftar Commands:\n\n" +
				"• /setjadwal [hari] [nama1, nama2, ...]\n" +
				"  Set jadwal akses untuk hari tertentu\n" +
				"  Contoh: /setjadwal senin ALVARO, AKBAR\n\n" +
				"• /lihatjadwal [hari]\n" +
				"  Lihat jadwal akses\n" +
				"  Contoh: /lihatjadwal senin\n" +
				"  Atau: /lihatjadwal (untuk semua hari)\n\n" +
				"• /database\n" +
				"  Lihat daftar pengguna dan UID dari database\n\n" +
				"• /help\n" +
				"  Tampilkan bantuan ini",
		)
	})

	b.Handle("/database", func(c telebot.Context) error {
		// Query admin users
		adminRows, err := db.Query("SELECT nama, uid FROM users WHERE is_admin = TRUE ORDER BY nama")
		if err != nil {
			return c.Send("❌ Error querying admin users: " + err.Error())
		}
		defer adminRows.Close()

		var adminUsers []struct {
			Nama string
			UID  string
		}
		for adminRows.Next() {
			var nama, uid string
			if err := adminRows.Scan(&nama, &uid); err != nil {
				continue
			}
			adminUsers = append(adminUsers, struct {
				Nama string
				UID  string
			}{nama, uid})
		}

		// Query non-admin users
		userRows, err := db.Query("SELECT nama, uid FROM users WHERE is_admin = FALSE ORDER BY nama")
		if err != nil {
			return c.Send("❌ Error querying users: " + err.Error())
		}
		defer userRows.Close()

		var regularUsers []struct {
			Nama string
			UID  string
		}
		for userRows.Next() {
			var nama, uid string
			if err := userRows.Scan(&nama, &uid); err != nil {
				continue
			}
			regularUsers = append(regularUsers, struct {
				Nama string
				UID  string
			}{nama, uid})
		}

		// Build response
		response := "📊 DATABASE USERS\n\n"
		response += "👑 ADMIN (" + strconv.Itoa(len(adminUsers)) + " orang):\n"
		if len(adminUsers) == 0 {
			response += "  (kosong)\n"
		} else {
			for _, user := range adminUsers {
				response += "  • " + user.Nama + " → " + user.UID + "\n"
			}
		}

		response += "\n👤 REGULAR (" + strconv.Itoa(len(regularUsers)) + " orang):\n"
		if len(regularUsers) == 0 {
			response += "  (kosong)\n"
		} else {
			for _, user := range regularUsers {
				response += "  • " + user.Nama + " → " + user.UID + "\n"
			}
		}

		return c.Send(response)
	})

	log.Println("✅ Telegram bot started, listening for commands...")

	// Jalankan di goroutine agar tidak block HTTP server
	go b.Start()

	// Kirim notifikasi bahwa server online ke grup
	if err := KirimNotifikasi(cfg, "🖥️ Door Lock Server online\n"+time.Now().Format("02-01-2006 15:04:05")); err != nil {
		log.Println("Warning: gagal kirim notifikasi startup ke Telegram:", err)
	}

	return nil
}
