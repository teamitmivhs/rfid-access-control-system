# Schedule Parser - Dokumentasi

Parser JSON ketat untuk sistem kontrol akses ruang server. Mengektrak jadwal piket dari format teks bebas dan mengkonversi ke JSON terstruktur.

## Aturan Parser

1. **Nama dikonversi ke UPPERCASE** - semua input diubah ke huruf besar
2. **Hanya nama dari input** - tidak menambahkan nama yang tidak ada
3. **Hari Indonesia** - Minggu, Senin, Selasa, Rabu, Kamis, Jumat, Sabtu
4. **Hari kosong = array kosong** - jika tidak disebutkan atau ditandai "-", "kosong", "libur", "off", "cuti"
5. **Bersihkan duplikat** - jika nama muncul 2x hari sama, hanya 1x diambil
6. **Bersihkan karakter** - abaikan spasi berlebih, titik, koma
7. **Pisahkan nama dengan koma**

## Contoh Input/Output

### Contoh 1: Format Sederhana
```
Input:
"Senin: ALVARO, FIKRI
Selasa: GANI
Rabu: -"

Output:
{
  "Minggu": [],
  "Senin": ["ALVARO", "FIKRI"],
  "Selasa": ["GANI"],
  "Rabu": [],
  "Kamis": [],
  "Jumat": [],
  "Sabtu": []
}
```

### Contoh 2: Nama Lowercase
```
Input:
"Senin: alvaro, fikri"

Output:
{
  "Minggu": [],
  "Senin": ["ALVARO", "FIKRI"],
  "Selasa": [],
  "Rabu": [],
  "Kamis": [],
  "Jumat": [],
  "Sabtu": []
}
```

### Contoh 3: Marker Kosong
```
Input:
"Senin: kosong
Selasa: ALVARO"

Output:
{
  "Minggu": [],
  "Senin": [],
  "Selasa": ["ALVARO"],
  "Rabu": [],
  "Kamis": [],
  "Jumat": [],
  "Sabtu": []
}
```

### Contoh 4: Duplikat & Spasi Berlebih
```
Input:
"Senin:  ALVARO  ,  FIKRI  ,  alvaro  .
Selasa: GANI, GHONI."

Output:
{
  "Minggu": [],
  "Senin": ["ALVARO", "FIKRI"],
  "Selasa": ["GANI", "GHONI"],
  "Rabu": [],
  "Kamis": [],
  "Jumat": [],
  "Sabtu": []
}
```

### Contoh 5: Hari Inggris
```
Input:
"Monday: ALVARO
Tuesday: FIKRI
Wednesday: -
Thursday: GANI, GHONI"

Output:
{
  "Minggu": [],
  "Senin": ["ALVARO"],
  "Selasa": ["FIKRI"],
  "Rabu": [],
  "Kamis": ["GANI", "GHONI"],
  "Jumat": [],
  "Sabtu": []
}
```

## API Endpoints

### 1. Parse Schedule dari Teks
**POST** `/api/parse-schedule`

Mengparse teks jadwal piket menjadi JSON terstruktur dan langsung update database.

#### Request
```json
{
  "text": "Senin: ALVARO, FIKRI\nSelasa: GANI\nRabu: -"
}
```

#### Response
```json
{
  "message": "Schedule parsed and updated successfully",
  "schedule": {
    "Minggu": [],
    "Senin": ["ALVARO", "FIKRI"],
    "Selasa": ["GANI"],
    "Rabu": [],
    "Kamis": [],
    "Jumat": [],
    "Sabtu": []
  }
}
```

#### Error Response
```json
{
  "error": "Text field is required"
}
```

### 2. Langsung Update Schedule
**POST** `/api/schedule`

Update jadwal dengan JSON terstruktur langsung (format sudah rapi).

#### Request
```json
{
  "Minggu": [],
  "Senin": ["ALVARO", "FIKRI"],
  "Selasa": ["GANI"],
  "Rabu": [],
  "Kamis": [],
  "Jumat": [],
  "Sabtu": []
}
```

#### Response
```json
{
  "message": "Schedule updated successfully"
}
```

### 3. Check Access
**POST** `/api/access`

Cek akses user berdasarkan UID dan jadwal.

#### Request
```json
{
  "uid": "F3B812BA"
}
```

#### Response (Granted)
```json
{
  "access": true,
  "nama": "ALVARO",
  "status": "granted"
}
```

#### Response (Denied)
```json
{
  "access": false,
  "nama": "ALVARO",
  "status": "denied"
}
```

#### Response (Not Found)
```json
{
  "access": false,
  "nama": "Unknown",
  "status": "card_not_found"
}
```

## Contoh cURL

### Parse dari Teks
```bash
curl -X POST http://localhost:8080/api/parse-schedule \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Senin: alvaro, fikri\nSelasa: gani\nRabu: libur"
  }'
```

### Update Langsung
```bash
curl -X POST http://localhost:8080/api/schedule \
  -H "Content-Type: application/json" \
  -d '{
    "Minggu": [],
    "Senin": ["ALVARO", "FIKRI"],
    "Selasa": ["GANI"],
    "Rabu": [],
    "Kamis": [],
    "Jumat": [],
    "Sabtu": []
  }'
```

### Check Access
```bash
curl -X POST http://localhost:8080/api/access \
  -H "Content-Type: application/json" \
  -d '{"uid": "F3B812BA"}'
```

## Penggunaan di Go

```go
import "doorlock-access-control/utils"

// Parse text to schedule
schedule := utils.ParseSchedule("Senin: ALVARO, FIKRI\nSelasa: GANI")

// Akses hasil
fmt.Println(schedule.Senin)  // Output: [ALVARO FIKRI]
fmt.Println(schedule.Selasa) // Output: [GANI]
```

## Supported Day Markers

### Indonesian
- Minggu, Senin, Selasa, Rabu, Kamis, Jumat, Sabtu

### English
- Sunday, Monday, Tuesday, Wednesday, Thursday, Friday, Saturday

## Supported Empty Markers
- "-"
- "kosong"
- "libur"
- "off"
- "cuti"

## Testing

Run unit tests:
```bash
go test -v ./utils
```

Tests mencakup:
- Format sederhana dengan colon
- Konversi lowercase ke uppercase
- Marker kosong (kosong, -, libur, off, cuti)
- Duplikat dalam hari sama
- Spasi berlebih dan punctuation
- Hari dalam bahasa Inggris
- Input kosong
- Hari dash marker
