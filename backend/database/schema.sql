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
-- Pengguna bisa akses hanya pada hari-hari yang dijadwalkan
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

-- Insert default settings
INSERT INTO settings (setting_key, setting_value) VALUES
('relay_open_duration', '2000'),
('telegram_enabled', 'true'),
('telegram_token', ''),
('telegram_chat_id', ''),
('door_name', 'Main Door Lock');

-- Sample data: Users dengan 33 kartu
INSERT INTO users (uid, nama) VALUES
('938934FF', 'ALVARO'),
('2DCC8C8B', 'AKBAR'),
('B9899911', 'DANI'),
('F3B812BA', 'FIKRI'),
('43358A01', 'GANI'),
('12988D8B', 'GHONI'),
('46D722BC', 'USER7'),
('B6C7FEBB', 'USER8'),
('03663C0E', 'USER9'),
('068343BC', 'USER10'),
('9B7EB350', 'USER11'),
('A6D6FCBB', 'USER12'),
('A3CD0903', 'USER13'),
('11111111', 'USER14'),
('22222222', 'USER15'),
('33333333', 'USER16'),
('44444444', 'USER17'),
('55555555', 'USER18'),
('66666666', 'USER19'),
('77777777', 'USER20'),
('88888888', 'USER21'),
('99999999', 'USER22'),
('AAAAAAAA', 'USER23'),
('BBBBBBBB', 'USER24'),
('CCCCCCCC', 'USER25'),
('DDDDDDDD', 'USER26'),
('EEEEEEEE', 'USER27'),
('FFFFFFFF', 'USER28'),
('00000001', 'USER29'),
('00000002', 'USER30'),
('00000003', 'USER31'),
('00000004', 'USER32'),
('00000005', 'USER33');

-- Sample schedule: Tiga pengguna pertama punya akses Senin-Jumat
INSERT INTO schedules (user_id, hari) VALUES
(1, 'Senin'), (1, 'Selasa'), (1, 'Rabu'), (1, 'Kamis'), (1, 'Jumat'),
(2, 'Senin'), (2, 'Selasa'), (2, 'Rabu'), (2, 'Kamis'), (2, 'Jumat'),
(3, 'Senin'), (3, 'Selasa'), (3, 'Rabu'), (3, 'Kamis'), (3, 'Jumat');
