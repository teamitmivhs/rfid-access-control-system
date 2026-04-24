CREATE DATABASE IF NOT EXISTS doorlock_db;
USE doorlock_db;

-- Tabel Users: Daftar semua pengguna kartu RFID
CREATE TABLE IF NOT EXISTS users (
	id INT PRIMARY KEY AUTO_INCREMENT,
	uid VARCHAR(20) UNIQUE NOT NULL,
	nama VARCHAR(100) NOT NULL,
	is_active BOOLEAN DEFAULT TRUE,
	is_admin BOOLEAN DEFAULT FALSE,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Tabel Schedule: Jadwal akses per hari (Senin-Jumat)
CREATE TABLE IF NOT EXISTS schedules (
	id INT PRIMARY KEY AUTO_INCREMENT,
	user_id INT NOT NULL,
	hari ENUM('Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat') NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
	UNIQUE KEY unique_schedule (user_id, hari)
);

-- Tabel Access Logs: Rekam setiap akses (granted/denied)
CREATE TABLE IF NOT EXISTS access_logs (
	id INT PRIMARY KEY AUTO_INCREMENT,
	user_id INT,
	uid VARCHAR(20) NOT NULL,
	nama VARCHAR(100) NOT NULL,
	status ENUM('GRANTED', 'DENIED', 'SCHEDULE_DENIED') NOT NULL,
	waktu TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
	INDEX idx_waktu (waktu),
	INDEX idx_status (status)
);

-- Tabel Settings: Konfigurasi sistem
CREATE TABLE IF NOT EXISTS settings (
	id INT PRIMARY KEY AUTO_INCREMENT,
	setting_key VARCHAR(100) UNIQUE NOT NULL,
	setting_value TEXT NOT NULL,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- FIX: Isi telegram_token dan telegram_chat_id dengan nilai yang sama seperti di ESP32
-- Dengan ini server bisa baca token dari DB, tidak perlu hardcode di kode Go
INSERT INTO settings (setting_key, setting_value) VALUES
('relay_open_duration', '2000'),
('telegram_enabled', 'true'),
('telegram_token', '8683423891:AAFTBmo3owh5sA0MGPgvX5IpZv3lI7iFYFc'),
('telegram_chat_id', '-1003302843795'),
('door_name', 'Main Door Lock'),
('device_type', 'ESP32'),
('device_name', 'RFID Door Lock System'),
('device_started_at', NOW()),
('device_last_heartbeat', NOW()),
('relay_status', '0')
ON DUPLICATE KEY UPDATE setting_value = VALUES(setting_value);

-- Sample data: Admin users
INSERT IGNORE INTO users (uid, nama, is_admin) VALUES
('938934FF', 'ALVARO', TRUE),
('2DCC8C8B', 'AKBAR', TRUE),
('83AE4305', 'JEKI', TRUE),
('EF76D91E', 'RAIHAN', TRUE),
('55E2FD52', 'HEAS', TRUE),
('0284BB1B', 'FERI', TRUE);

-- Regular users (scheduled access)
INSERT IGNORE INTO users (uid, nama, is_admin) VALUES
('B9899911', 'DANI', FALSE),
('B9C87112', 'IHSAN', FALSE),
('D36C4605', 'FAAIZ', FALSE),
('12988D8B', 'GHONI', FALSE);

-- Sample schedule
INSERT IGNORE INTO schedules (user_id, hari) VALUES
(7, 'Senin'), (7, 'Selasa'), (7, 'Rabu'), (7, 'Kamis'), (7, 'Jumat'),
(8, 'Senin'), (8, 'Selasa'), (8, 'Rabu'), (8, 'Kamis'), (8, 'Jumat'),
(9, 'Senin'), (9, 'Selasa'), (9, 'Rabu'), (9, 'Kamis'), (9, 'Jumat');