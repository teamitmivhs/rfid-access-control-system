# RFID Door Lock System - Backend & Setup

Sistem kontrol akses pintu otomatis dengan RFID card reader, relay control untuk magnetic lock, dan manajemen jadwal akses berbasis database.

## 📋 Daftar Isi
1. [Setup Database](#setup-database)
2. [Wiring ESP32](#wiring-esp32)
3. [Konfigurasi Backend](#konfigurasi-backend)
4. [Troubleshooting](#troubleshooting-masalah-umum)

---

## Setup Database

### Install MySQL/MariaDB
```bash
# Windows (pastikan sudah install MySQL server)
mysql -u root -p

# Linux
sudo apt-get install mysql-server

# macOS
brew install mysql
```

### Buat Database
```bash
mysql -u root -p < backend/database/schema.sql
```

Database akan berisi:
- **users**: 33 pengguna dengan UID kartu RFID
- **schedules**: Jadwal akses Senin-Jumat
- **access_logs**: Rekam setiap akses
- **settings**: Konfigurasi Telegram token, relay duration, dll

---

## Wiring ESP32

### GPIO Mapping (ESP32 DevKit)

| Fungsi | GPIO | Tujuan |
|--------|------|--------|
| **Relay Control** | 4 | Magnetic Lock |
| **Manual Button** | 15 | Tombol Buka Pintu |
| **Buzzer** | 13 | Audio Feedback |
| **RFID RST** | 22 | MFRC522 Reset |
| **RFID SS/CS** | 21 | MFRC522 Chip Select |
| **RFID SCK** | 18 | SPI Clock |
| **RFID MISO** | 19 | SPI Data In |
| **RFID MOSI** | 23 | SPI Data Out |
| **Reed Switch** | 26 | Door Status (optional) |

### Wiring Hardware

#### 🔴 Relay Module (5V)
```
Relay VCC  → ESP32 5V (via USB atau external power)
Relay GND  → ESP32 GND
Relay IN   → ESP32 GPIO 4 (HIGH = ON, LOW = OFF)
Relay COM  → Magnetic Lock +12V
Relay NO   → Magnetic Lock live dari power supply 12V
```
**Catatan**: Jangan koneksi relay IN langsung ke 5V! Harus via GPIO4.

#### 🟢 MFRC522 RFID Reader (3.3V)
```
MFRC522 VCC → ESP32 3.3V
MFRC522 GND → ESP32 GND
MFRC522 RST → ESP32 GPIO 22
MFRC522 SS  → ESP32 GPIO 21
MFRC522 SCK → ESP32 GPIO 18
MFRC522 MOSI → ESP32 GPIO 23
MFRC522 MISO → ESP32 GPIO 19
```
**Penting**: RFID perlu 3.3V eksak, jangan pakai 5V!

#### 🟡 Buzzer (5V)
```
Buzzer +  → ESP32 GPIO 13
Buzzer -  → ESP32 GND (via 1k resistor)
```
atau langsung ke GND kalau Buzzer aktif LOW.

#### 🔵 Manual Button (Push Button)
```
Button Pin 1 → ESP32 GPIO 15
Button Pin 2 → ESP32 GND
```
Button aktif LOW (INPUT_PULLUP internal).

#### 🟣 Reed Switch (Door Sensor) - Optional
```
Reed Switch → ESP32 GPIO 26 + GND
```

---

## Konfigurasi Backend

### Install Dependencies
```bash
cd backend
go mod download
```

### Konfigurasi Database
Edit `backend/config/database.go`:
```go
const (
    DBUser     = "root"
    DBPassword = "your_password"
    DBHost     = "localhost"
    DBPort     = "3306"
    DBName     = "doorlock_db"
)
```

### Konfigurasi Telegram Bot (Optional)
Buka database dan update settings:
```sql
UPDATE settings SET setting_value = '8683423891:AAFTBmo3owh5sA0MGPgvX5IpZv3lI7iFYFc' 
WHERE setting_key = 'telegram_token';

UPDATE settings SET setting_value = '-1003302843795' 
WHERE setting_key = 'telegram_chat_id';
```

### Build & Run
```bash
# Build backend
go build -o doorlock-server main.go

# Run
./doorlock-server
```

Backend akan berjalan di `http://localhost:8080`

---

## API Endpoints

### Verify Access
```
POST /api/access/verify
{
  "uid": "938934FF",
  "timestamp": "2026-04-11T14:30:00Z"
}

Response (Allowed):
{
  "allowed": true,
  "name": "ALVARO",
  "message": "Access Granted"
}

Response (Denied):
{
  "allowed": false,
  "reason": "Schedule not allowed"
}
```

---

## Troubleshooting: Masalah Umum

### ⚠️ ESP32 Gagal Upload

**Masalah**: `Failed to connect to ESP32: Wrong boot mode detected (0xb)`

**Penyebab**: 
- GPIO 0 floating saat boot
- GPIO 2 kena pull-down dari hardware (GPIO 2 adalah boot pin)
- USB Cable lepas atau contact buruk

**Solusi**:
1. Sambungkan GPIO 0 ke GND dengan push button atau micro switch
2. Tekan tombol sambil power cycle ESP32 → lepas tombol
3. Retry upload

Atau gunakan HARD RESET button di GPIO 0 yang sudah kami implementasikan.

---

### ⚠️ RFID Reader Tidak Terbaca

**Masalah**: Serial monitor menunjukkan `ERROR: RFID tidak terdeteksi!`

**Penyebab**:
- Tegangan RFID tidak stabil (harus 3.3V eksak)
- Kabel SPI terlalu panjang atau ada EMI (electrical noise)
- Pin SS/CS tidak terhubung atau salah
- MFRC522 sudah rusak

**Solusi**:
1. Periksa tegangan MFRC522 dengan multimeter → harus 3.3V
2. Gunakan kabel SPI yang pendek (< 30cm)
3. Tambahkan ferrite core pada kabel SPI jika masih noise
4. Periksa semua sambungan SPI (SCK, MOSI, MISO, SS, RST)
5. Coba ganti MFRC522 dengan yang baru

---

### ⚠️ Relay Tidak Click/Tidak Ada Suara

**Masalah**: GPIO 4 keluar sinyal tapi relay tidak mengaktifkan magnetic lock

**Penyebab**:
- Relay VCC tidak terhubung ke power 5V
- Relay IN pin tidak terhubung ke GPIO 4
- Relay rusak atau tidak kompatibel
- Power supply 5V tidak cukup (relay butuh ~180mA minimum)

**Solusi**:
1. Cek tegangan relay VCC pakai multimeter → harus 5V
2. Cek GPIO 4 output dengan LED test (LED akan nyala/mati sesuai relay trigger)
3. Dengarkan suara relay saat tombol ditekan:
   - Ada suara "click" = relay baik, masalah di magnetic lock wiring
   - Tidak ada suara = relay bermasalah
4. Gunakan external 5V power supply (minimum 1A), jangan hanya dari USB

---

### ⚠️ Serial Monitor Blank / Tidak Ada Log

**Masalah**: Serial monitor tidak menampilkan apapun meski ESP32 sudah programmed

**Penyebab**:
- Baud rate salah (harus 9600)
- COM port salah dipilih
- USB driver CH340/CP2102 tidak terinstall
- TX/RX tidak terhubung (tapi ini tidak mungkin kalau semua port muncul)

**Solusi**:
1. Platform IO: Monitor Speed di `platformio.ini` harus 9600
   ```ini
   monitor_speed = 9600
   ```

2. Arduino IDE: Tools → Serial Monitor → 9600 baud

3. Install driver USB:
   - CH340: https://wa.fun/ch340 
   - CP2102: https://www.silabs.com/developers/usb-to-uart-bridge-vcp-drivers

4. Pastikan USB cable adalah data cable, bukan charging cable saja

---

### ⚠️ Tombol Manual Tidak Response

**Masalah**: Tekan tombol GPIO 15 tapi relay tidak aktif

**Penyebab**:
- GPIO 15 pin tidak terhubung ke push button
- Push button switch rusak
- Pull-up resistor internal tidak aktif di setting
- Debounce time terlalu lama

**Solusi**:
1. Test GPIO 15 dengan push button + LED:
   - Saat tombol ditekan, LED harus nyala
2. Periksa setting `pinMode(MANUAL_BTN_PIN, INPUT_PULLUP)` di setup()
3. Ganti push button dengan yang baru jika switch sudah aus
4. Durasi tekanan minimal 100ms (jangan tapped terlalu cepat)

---

### ⚠️ Magnetic Lock Tidak Mengunci/Membuka

**Masalah**: Relay aktif tapi magnetic lock tetap locked atau selalu terbuka

**Penyebab**:
- Relay COM/NO pin salah terhubung ke magnetic lock
- Magnetic lock butuh tegangan 12V tapi dapat < 10V
- Relay adalah NC (Normally Closed) bukan NO (Normally Open)
- Kabelnya terputus atau korosi

**Solusi**:
1. Ukur tegangan magnetic lock dengan multimeter:
   - Saat relay OFF → harus 0V
   - Saat relay ON → harus 12V
2. Periksa jenis relay:
   - NO = Normally Open (default terbuka, ON saat trigger)
   - NC = Normally Closed (default tertutup, OFF saat trigger)
3. Gunakan relay 12V 1 channel 30A minimum untuk magnetic lock 12V
4. Tambahkan diode 1N4007 across magnetic lock (anode ke -, katode ke +) untuk proteksi back-EMF

---

### ⚠️ WiFi Tidak Connect

**Masalah**: ESP32 tidak konek ke WiFi, log menampilkan dots (......) terus

**Penyebab**:
- SSID atau password salah di kode
- WiFi 5GHz (ESP32 hanya support 2.4GHz)
- WiFi signal lemah
- WiFi router tidak dalam jangkauan

**Solusi**:
1. Edit di `doorlock.cpp`:
   ```cpp
   const char* ssid     = "NAMA_WIFI";
   const char* password = "PASSWORD";
   ```

2. Pastikan WiFi router support 2.4GHz (B, G, N)

3. Cek signal strength di lokasi ESP32 → minimal -67 dBm

4. Coba restart router atau pasang extender WiFi

---

### ⚠️ Buzzer Tidak Berbunyi

**Masalah**: GPIO 13 aktif tapi buzzer tidak bunyi

**Penyebab**:
- Buzzer tidak terhubung atau GND terputus
- Buzzer polaritas terbalik
- Buzzer 12V butuh transistor amplifier, tidak bisa langsung dari GPIO

**Solusi**:
1. Gunakan buzzer passive 5V (yang butuh signal square wave)
2. Atau gunakan buzzer active 5V (butuh VCC + GND + SIGNAL)
3. Tambahkan transistor 2N2222 jika butuh buzzer 12V
4. Kurangi delay / jeda untuk membuat bunyi lebih terasa

---

### 💡 Tips Setup Aman
1. **Test satu-satu**: Test relay dulu, baru RFID, baru buzzer, baru tombol
2. **Pakai breadboard**: Jangan langsung solder ke ESP32
3. **Check multimeter**: Tegangan harus sesuai spek (3.3V untuk sensor, 5V untuk relay)
4. **Dekupulasi capasitor**: Tambahkan 100µF capasitor di dekat VCC relay untuk stabilitas
5. **Hindari ground loop**: Pastikan semua component punya GND yang sama
6. **Test database**: Curl `/api/access/verify` sebelum ESP32 bikin request

---

## Folder Structure

```
project pintu IT/
├── backend/                    # Server Go + Database
│   ├── main.go                 # Entry point server
│   ├── config/database.go      # Database connection
│   ├── handlers/               # API endpoints
│   ├── models/                 # Data structures
│   ├── utils/                  # Helper functions
│   ├── database/schema.sql     # Database schema
│   └── README.md               # Setup guide
│
└── src/                        # Firmware & dokumentasi
    ├── esp32/doorlock.cpp      # ESP32 firmware (Arduino)
    └── docs/architecture.md    # System architecture
```

---

## Contact & Support

Jika masih ada masalah:
1. Periksa log di Serial Monitor (9600 baud)
2. Lihat access_logs di database untuk debug
3. Enable Telegram notification untuk real-time alert

Good luck! 🚀
