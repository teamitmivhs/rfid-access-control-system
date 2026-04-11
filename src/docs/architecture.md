# Arsitektur Sistem Door Lock RFID

## Overview
Sistem kontrol akses pintu berbasis RFID dengan manajemen jadwal akses, logging, dan integrasi Telegram.

## Komponen Utama

### 1. Hardware (ESP32)
- **RFID Reader**: Membaca UID kartu RFID via SPI (MFRC522)
- **Relay Control**: Mengontrol magnetic lock (GPIO 4)
- **Manual Button**: Tombol buka pintu dengan auto-lock 2 detik (GPIO 15)
- **Buzzer**: Feedback audio (GPIO 13)
- **WiFi**: Koneksi ke backend server
- **NTP Time Sync**: Sinkronisasi waktu dari internet

### 2. Backend (Go)
- **REST API**: Endpoint untuk validasi akses, jadwal, dan logging
- **Database**: MySQL dengan tabel users, schedules, access_logs
- **Telegram Bot**: Notifikasi akses real-time

### 3. Database Structure
```
users          → Daftar pengguna + UID kartu
schedules      → Jadwal akses per hari (Senin-Jumat)
access_logs    → Rekam semua akses (granted/denied)
settings       → Konfigurasi sistem
```

## Flow Akses

### Skenario 1: RFID Card Scanned
```
Card Detected
    ↓
Read UID
    ↓
Query Database (users table)
    ↓
Found? → Check Schedule (sesuai hari)
    ├─ Schedule OK → RELAY ON → Log GRANTED → Beep OK → Auto-lock 2s
    └─ Schedule NO → RELAY OFF → Log DENIED → Beep Error → Telegram Notify
    
Not Found? → RELAY OFF → Log DENIED → Beep Error → Telegram Notify
```

### Skenario 2: Manual Button (GPIO 15)
```
Button Pressed (GPIO 15)
    ↓
RELAY ON (HIGH) → Magnetic Lock Powered
    ↓
Wait 2 seconds (RELAY_HOLD_TIME = 2000ms)
    ↓
RELAY OFF (LOW) → Magnetic Lock Locked
    ↓
Buzzer Beep x2 (Konfirmasi auto-lock)
```

## Koneksi ESP32-Backend
- **REST API Call**: Validasi UID ke API `/api/access/verify`
- **Request**: `POST /api/access/verify` denganUID + waktu akses
- **Response**: 
  - `{allowed: true, name: "..."}` → Buka pintu
  - `{allowed: false, reason: "..."}` → Tolak akses

## Logging
Setiap akses dicatat dengan:
- UID kartu
- Nama pengguna
- Status (GRANTED/DENIED/SCHEDULE_DENIED)
- Timestamp
- Dikirim ke Telegram jika enabled

## Jadwal Akses
- Periode: **Senin - Jumat** (5 hari kerja)
- Konfigurasi per user di database `schedules` table
- Sistem otomatis cek hari saat ini (NTP synced)

## Fitur Keamanan
✓ Validasi UID di database
✓ Pengecekan jadwal akses harian
✓ Daftar hitam (is_active flag)
✓ Logging audit trail
✓ Notifikasi Telegram real-time
✓ OTA firmware update
