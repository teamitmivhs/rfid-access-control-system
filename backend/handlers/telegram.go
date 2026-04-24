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
	adminSkippedNames := []string{}

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || name == "-" || strings.ToLower(name) == "kosong" {
			continue
		}
		nameUpper := strings.ToUpper(name)

		var userID int
		var isAdmin bool
		err := db.QueryRow(
			"SELECT id, is_admin FROM users WHERE UPPER(nama) = ? AND is_active = TRUE",
			nameUpper,
		).Scan(&userID, &isAdmin)
		if err != nil {
			notFoundNames = append(notFoundNames, nameUpper)
			continue
		}

		// Skip jika user adalah admin
		if isAdmin {
			adminSkippedNames = append(adminSkippedNames, nameUpper)
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
	if len(adminSkippedNames) > 0 {
		response += "\n⚠️  Tidak perlu dijadwal (" + strconv.Itoa(len(adminSkippedNames)) + " orang - sudah admin):\n"
		for _, n := range adminSkippedNames {
			response += "  • " + n + " (tidak perlu ditambahkan, sudah menjadi admin dan bisa akses kapan saja)\n"
		}
	}
	if len(notFoundNames) > 0 {
		response += "\n❌ Tidak ditemukan di database (" + strconv.Itoa(len(notFoundNames)) + " orang):\n"
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

// DeviceStatusHandler: Handle command /device
// Menampilkan status perangkat ESP32 termasuk:
// - Jenis perangkat (ESP32)
// - Status online/offline
// - Uptime
// - Informasi sistem lainnya
func DeviceStatusHandler(db *sql.DB, c telebot.Context) error {
	// Baca device settings dari database
	deviceSettings := make(map[string]string)
	requiredSettings := []string{
		"device_type", "device_name", "device_started_at",
		"device_last_heartbeat", "relay_status", "door_name",
	}

	for _, key := range requiredSettings {
		var value string
		err := db.QueryRow(
			"SELECT setting_value FROM settings WHERE setting_key = ?",
			key,
		).Scan(&value)
		if err == nil {
			deviceSettings[key] = value
		}
	}

	// Set default values jika belum ada
	if deviceSettings["device_type"] == "" {
		deviceSettings["device_type"] = "ESP32"
	}
	if deviceSettings["device_name"] == "" {
		deviceSettings["device_name"] = "RFID Door Lock System"
	}
	if deviceSettings["door_name"] == "" {
		deviceSettings["door_name"] = "Main Door"
	}

	// Hitung uptime berdasarkan device_started_at
	uptime := "Unknown"
	if deviceSettings["device_started_at"] != "" {
		startTime, err := time.Parse("2006-01-02 15:04:05", deviceSettings["device_started_at"])
		if err == nil {
			duration := time.Since(startTime)
			days := int(duration.Hours() / 24)
			hours := int(duration.Hours()) % 24
			minutes := int(duration.Minutes()) % 60

			if days > 0 {
				uptime = fmt.Sprintf("%d hari %d jam %d menit", days, hours, minutes)
			} else if hours > 0 {
				uptime = fmt.Sprintf("%d jam %d menit", hours, minutes)
			} else {
				uptime = fmt.Sprintf("%d menit", minutes)
			}
		}
	}

	// Tentukan status device berdasarkan last heartbeat
	deviceStatus := "🔴 OFFLINE"
	lastHeartbeat := "Tidak ada data"
	if deviceSettings["device_last_heartbeat"] != "" {
		lastHeartbeatTime, err := time.Parse("2006-01-02 15:04:05", deviceSettings["device_last_heartbeat"])
		if err == nil {
			lastHeartbeat = lastHeartbeatTime.Format("02-01-2006 15:04:05")

			// Jika last heartbeat kurang dari 5 menit yang lalu, status ONLINE
			if time.Since(lastHeartbeatTime) < 5*time.Minute {
				deviceStatus = "🟢 ONLINE"
			} else if time.Since(lastHeartbeatTime) < 1*time.Hour {
				deviceStatus = "🟡 IDLE (Terakhir aktif: " + fmt.Sprintf("%d menit", int(time.Since(lastHeartbeatTime).Minutes())) + " lalu)"
			}
		}
	}

	// Hitung total akses hari ini
	var totalAccessToday int
	today := time.Now().Format("2006-01-02")
	err := db.QueryRow(
		"SELECT COUNT(*) FROM access_logs WHERE DATE(waktu) = ?",
		today,
	).Scan(&totalAccessToday)
	if err != nil {
		totalAccessToday = 0
	}

	// Hitung access granted vs denied hari ini
	var grantedToday, deniedToday int
	db.QueryRow(
		"SELECT COUNT(*) FROM access_logs WHERE DATE(waktu) = ? AND status = 'GRANTED'",
		today,
	).Scan(&grantedToday)
	db.QueryRow(
		"SELECT COUNT(*) FROM access_logs WHERE DATE(waktu) = ? AND status = 'DENIED'",
		today,
	).Scan(&deniedToday)

	// Hitung total akses user
	var totalUsers int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE is_active = TRUE").Scan(&totalUsers)

	// Relay status
	relayStatus := "Unknown"
	if deviceSettings["relay_status"] != "" {
		if deviceSettings["relay_status"] == "1" || deviceSettings["relay_status"] == "open" {
			relayStatus = "🔓 Terbuka"
		} else {
			relayStatus = "🔒 Tertutup"
		}
	}

	// Build response
	response := "🖥️ STATUS PERANGKAT\\n"
	response += "═════════════════════════════════════\\n\\n"

	// Device Info
	response += fmt.Sprintf("📱 Jenis Perangkat: %s\\n", deviceSettings["device_type"])
	response += fmt.Sprintf("💻 Nama: %s\\n", deviceSettings["device_name"])
	response += fmt.Sprintf("🚪 Pintu: %s\\n\\n", deviceSettings["door_name"])

	// Status & Uptime
	response += fmt.Sprintf("Status: %s\\n", deviceStatus)
	response += fmt.Sprintf("⏱️ Uptime: %s\\n", uptime)
	response += fmt.Sprintf("🔔 Terakhir Aktif: %s\\n\\n", lastHeartbeat)

	// Door Status
	response += fmt.Sprintf("🔐 Status Pintu: %s\\n\\n", relayStatus)

	// Today Statistics
	response += fmt.Sprintf("📊 Statistik Hari Ini (%s):\\n", today)
	response += fmt.Sprintf("  ✅ Granted: %d akses\\n", grantedToday)
	response += fmt.Sprintf("  ❌ Denied: %d akses\\n", deniedToday)
	response += fmt.Sprintf("  📈 Total: %d akses\\n\\n", totalAccessToday)

	// System Info
	response += fmt.Sprintf("👥 Total User: %d (aktif)\\n", totalUsers)
	response += fmt.Sprintf("⚙️ Server Time: %s", time.Now().Format("02-01-2006 15:04:05"))

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
				"• /device\n" +
				"  Lihat status dan informasi perangkat ESP\n\n" +
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

	b.Handle("/device", func(c telebot.Context) error {
		return DeviceStatusHandler(db, c)
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
