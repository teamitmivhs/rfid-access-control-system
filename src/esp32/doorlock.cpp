#include <SPI.h>
#include <MFRC522.h>
#include <WiFi.h>
#include <HTTPClient.h>
#include <ArduinoOTA.h>
#include <ArduinoJson.h>
#include <time.h>

void bener();
void salah();
void buka();
void kirimPesan(String pesan);
String getWaktuDanTanggal();
String getHari();
String urlEncode(String str);
bool verifyAccessLocal(String uid, String &nama);
bool verifyAccessServer(String uid, String &nama);
void syncCardsFromServer();

const char* ssid     = "TEAM IT MIVHS";
const char* password = "1TM1TR4101101MIVHS2025PASTIBISA2026SELALULANCAR2027GENERASI2028";
const char* API_HOST = "192.168.107.37"; 
const int   API_PORT = 8081;

String BOT_TOKEN = "8683423891:AAFTBmo3owh5sA0MGPgvX5IpZv3lI7iFYFc";
String CHAT_ID   = "-1003302843795";

#define BUZ_PIN        2
#define RLY_PIN        3
#define MANUAL_BTN_PIN 1
#define LRM_PIN        8 
#define SPI_SCK   18
#define SPI_MISO  19
#define SPI_MOSI  23
#define SS_PIN    5
#define RST_PIN   22
#define RELAY_ON  HIGH
#define RELAY_OFF LOW

int   jeda2           = 200;
int   jeda1           = jeda2;
bool  logika1         = false;
bool  logika3         = false;
float lama_buka_pintu = 2.0;
unsigned long waktu   = 0;

// Manual button variables
bool manual_btn_state             = true;
bool manual_btn_prev              = true;
unsigned long manual_btn_debounce = 0;
unsigned long relay_active_time   = 0;
bool relay_is_active              = false;
const unsigned long DEBOUNCE_DELAY = 50;
const unsigned long RELAY_HOLD_TIME = 2000;

//kartu admin (Always allowed)
const int jumlah_kartu = 36;
const String daftarUID[jumlah_kartu] = {
  "938934FF",  // ALVARO
  "2DCC8C8B",  // AKBAR
  "83AE4305",  // JEKI
  "EF76D91E",  // RAIHAN
  "55E2FD52",  // HEAS
  "0284BB1B",  // FERI
};

const String daftarNama[jumlah_kartu] = {
  "ALVARO (ADMIN)",
  "AKBAR (ADMIN)",
  "JEKI (ADMIN)",
  "RAIHAN (ADMIN)",
  "HEAS (ADMIN)",
  "FERI (ADMIN)",
};

//kartu yang diambil dari server
const int MAX_SERVER_CARDS = 100;
String serverCardUID[MAX_SERVER_CARDS];
String serverCardNama[MAX_SERVER_CARDS];
int serverCardCount = 0;
unsigned long lastSyncTime = 0;
const unsigned long SYNC_INTERVAL = 3600000; // Sync setiap 1 jam

MFRC522 mfrc522(SS_PIN, RST_PIN);

// Verifikasi kartu secara lokal (Admin - offline-first)
bool verifyAccessLocal(String uid, String &nama) {
  for (int i = 0; i < jumlah_kartu; i++) {
    if (uid == daftarUID[i]) {
      nama = daftarNama[i];
      Serial.println("✓ Kartu ADMIN cocok: " + nama);
      return true;
    }
  }
  Serial.println("Kartu tidak ada di daftar admin");
  return false;
}

// Verifikasi kartu dari server cache (scheduled untuk hari ini)
bool verifyAccessServer(String uid, String &nama) {
  if (serverCardCount == 0) {
    Serial.println("⚠️  Tidak ada kartu dari server untuk hari ini");
    return false;
  }

  for (int i = 0; i < serverCardCount; i++) {
    if (uid == serverCardUID[i]) {
      nama = serverCardNama[i];
      Serial.println("✓ Kartu SCHEDULED cocok: " + nama);
      return true;
    }
  }
  Serial.println("Kartu tidak di jadwal hari ini");
  return false;
}

// Sync kartu dari server untuk hari ini
// BUG FIX: hapus guard duplikat di dalam fungsi — pengecekan interval cukup di loop()
void syncCardsFromServer() {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("⚠️  WiFi not connected, skipping sync");
    return;
  }

  Serial.println("\n🔄 Syncing scheduled cards from server...");

  HTTPClient http;
  http.setTimeout(2000); // FIX: timeout diperkecil agar tidak ngeblok loop lama
  
  // BUG FIX: endpoint diganti ke /api/cards/scheduled-today
  // agar server hanya mengembalikan scheduled cards (bukan admin)
  String url = "http://" + String(API_HOST) + ":" + String(API_PORT) + "/api/cards/scheduled-today";
  
  if (!http.begin(url)) {
    Serial.println("❌ Failed to begin HTTP connection");
    http.end();
    return;
  }

  int httpCode = http.GET();
  
  if (httpCode != 200) {
    Serial.println("❌ HTTP Error: " + String(httpCode));
    http.end();
    return;
  }

  String response = http.getString();
  http.end();

  DynamicJsonDocument doc(2048);
  DeserializationError error = deserializeJson(doc, response);

  if (error) {
    Serial.println("❌ JSON parse error: " + String(error.c_str()));
    return;
  }

  serverCardCount = 0;

  JsonArray cards = doc["cards"].as<JsonArray>();
  
  int count = 0;
  for (JsonObject card : cards) {
    if (count >= MAX_SERVER_CARDS) break;
    
    String uid = card["uid"].as<String>();
    String nama = card["nama"].as<String>();

    serverCardUID[count] = uid;
    serverCardNama[count] = nama;
    count++;
  }

  serverCardCount = count;
  // FIX: lastSyncTime sudah di-set di loop() sebelum fungsi ini dipanggil

  Serial.println("✓ Sync complete! " + String(serverCardCount) + " scheduled cards loaded for " + doc["hari"].as<String>());
}

void setup() {
  Serial.begin(9600);
  delay(1000);
  Serial.println("\n\n=== DOOR LOCK STARTING ===");
  
  pinMode(RLY_PIN, OUTPUT);
  
  Serial.println("\n*** RELAY DIAGNOSTIC TEST ***");
  Serial.print("RLY_PIN: ");
  Serial.println(RLY_PIN);
  Serial.print("RELAY_ON (HIGH): ");
  Serial.println(RELAY_ON);
  Serial.print("RELAY_OFF (LOW): ");
  Serial.println(RELAY_OFF);
  
  digitalWrite(RLY_PIN, RELAY_OFF);
  delay(500);
  Serial.print("Relay state after RELAY_OFF: ");
  Serial.println(digitalRead(RLY_PIN));
  
  Serial.println("\nToggle relay 3x untuk test GPIO...");
  for (int i = 0; i < 3; i++) {
    Serial.print("Toggle "); Serial.print(i + 1); Serial.print(" - ON: ");
    digitalWrite(RLY_PIN, RELAY_ON);
    delay(300);
    Serial.println(digitalRead(RLY_PIN));
    
    Serial.print("Toggle "); Serial.print(i + 1); Serial.print(" - OFF: ");
    digitalWrite(RLY_PIN, RELAY_OFF);
    delay(300);
    Serial.println(digitalRead(RLY_PIN));
  }
  
  Serial.println("*** END DIAGNOSTIC TEST ***\n");

  pinMode(MANUAL_BTN_PIN, INPUT_PULLUP);
  pinMode(BUZ_PIN, OUTPUT);
  digitalWrite(BUZ_PIN, LOW);

  SPI.begin(SPI_SCK, SPI_MISO, SPI_MOSI, SS_PIN);
  mfrc522.PCD_Init();
  delay(50);

  byte ver = mfrc522.PCD_ReadRegister(MFRC522::VersionReg);
  Serial.print("MFRC522 version: 0x");
  Serial.println(ver, HEX);
  if (ver == 0x00 || ver == 0xFF) {
    Serial.println("ERROR: RFID tidak terdeteksi! Cek wiring SPI.");
  } else {
    Serial.println("RFID OK");
  }

  Serial.println("\n*** Relay Verification ***");
  for (int i = 0; i < 3; i++) {
    Serial.print("Check "); Serial.print(i + 1); Serial.print(": Relay pin state = ");
    Serial.println(digitalRead(RLY_PIN));
    if (digitalRead(RLY_PIN) != LOW) {
      Serial.println("WARNING: Relay not OFF! Trying to set OFF again...");
      digitalWrite(RLY_PIN, RELAY_OFF);
      delay(500);
    }
  }
  Serial.println("*** Relay OK ***\n");

  //ESP32 static ip address
  IPAddress local_IP(192, 168, 107, 100);
  IPAddress gateway(192, 168, 96, 1);
  IPAddress subnet(255, 255, 240, 0);  // /20 subnet mask
  IPAddress dns(8, 8, 8, 8);
  if (!WiFi.config(local_IP, gateway, subnet, dns)) {
    Serial.println("WARNING: Failed to configure static IP, using DHCP");
  } else {
    Serial.println("Static IP configured: 192.168.107.100");
  }
//ESP32 wifi conf begin 
  WiFi.begin(ssid, password);
  Serial.print("Connecting to WiFi");
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println("\nConnected!");
  Serial.println(WiFi.localIP());

  configTime(7 * 3600, 0, "pool.ntp.org", "time.nist.gov");
  Serial.println("Waiting for NTP time sync...");
  time_t now = time(nullptr);
  while (now < 24 * 3600) {
    delay(500);
    Serial.print(".");
    now = time(nullptr);
  }
  Serial.println("\nTime synced!");

  Serial.println("\n📥 Syncing scheduled cards from server...");
  syncCardsFromServer();

  kirimPesan("Door LOCK online\n" + getWaktuDanTanggal() + "\n" + getHari());

  ArduinoOTA.setHostname("DoorLock-ESP32");
  ArduinoOTA.onStart([]() { Serial.println("Start OTA update"); });
  ArduinoOTA.onEnd([]() { Serial.println("\nEnd OTA"); });
  ArduinoOTA.onProgress([](unsigned int progress, unsigned int total) {
    Serial.printf("OTA Progress: %u%%\r", (progress / (total / 100)));
  });
  ArduinoOTA.onError([](ota_error_t error) {
    Serial.printf("OTA Error[%u]: ", error);
    if      (error == OTA_AUTH_ERROR)    Serial.println("Auth Failed");
    else if (error == OTA_BEGIN_ERROR)   Serial.println("Begin Failed");
    else if (error == OTA_CONNECT_ERROR) Serial.println("Connect Failed");
    else if (error == OTA_RECEIVE_ERROR) Serial.println("Receive Failed");
    else if (error == OTA_END_ERROR)     Serial.println("End Failed");
  });
  ArduinoOTA.begin();
}

void loop() {
  ArduinoOTA.handle();

  // FIX: set lastSyncTime DULU sebelum sync, agar kalau gagal/timeout
  // tidak langsung retry lagi di iterasi loop berikutnya
  if (WiFi.status() == WL_CONNECTED) {
    if (lastSyncTime == 0 || (millis() - lastSyncTime >= SYNC_INTERVAL)) {
      lastSyncTime = millis();
      syncCardsFromServer();
    }
  }

  // Manual button logic
  manual_btn_state = digitalRead(MANUAL_BTN_PIN);

  if (manual_btn_state == LOW && manual_btn_prev == HIGH) {
    manual_btn_debounce = millis();
  }

  if (manual_btn_state == LOW && (millis() - manual_btn_debounce >= DEBOUNCE_DELAY)) {
    if (manual_btn_prev == HIGH) {
      Serial.println("Tombol ditekan - Membuka pintu");
      digitalWrite(BUZ_PIN, HIGH); delay(100);
      digitalWrite(BUZ_PIN, LOW);

      digitalWrite(RLY_PIN, RELAY_ON);
      relay_is_active   = true;
      relay_active_time = millis();
      Serial.println("Relay ON - Pintu TERBUKA");
    }
  }
  manual_btn_prev = manual_btn_state;

  // Auto-lock setelah 2 detik
  if (relay_is_active && (millis() - relay_active_time >= RELAY_HOLD_TIME)) {
    digitalWrite(RLY_PIN, RELAY_OFF);
    relay_is_active = false;
    Serial.println("Relay OFF - Pintu TERKUNCI (auto-lock)");
    digitalWrite(BUZ_PIN, HIGH); delay(50);
    digitalWrite(BUZ_PIN, LOW);  delay(50);
    digitalWrite(BUZ_PIN, HIGH); delay(50);
    digitalWrite(BUZ_PIN, LOW);
  }

  // LRM logic
  if (!logika1 && digitalRead(LRM_PIN) == HIGH) {
    if (waktu == 0) waktu = millis();
    else if (millis() - waktu >= 11000 && !logika3) logika3 = true;
  }
  if (logika3) {
    digitalWrite(BUZ_PIN, HIGH); delay(jeda1);
    digitalWrite(BUZ_PIN, LOW);  delay(jeda1);
    if (jeda1 >= 70) jeda1 -= 1;
  }
  if (digitalRead(LRM_PIN) == LOW) {
    waktu = 0;
    if (logika3) { logika3 = false; jeda1 = jeda2; }
  }

  if (!mfrc522.PICC_IsNewCardPresent() || !mfrc522.PICC_ReadCardSerial()) return;

  String kartu = "";
  Serial.print(F("Card UID: "));
  for (byte i = 0; i < mfrc522.uid.size; i++) {
    kartu.concat(String(mfrc522.uid.uidByte[i] < 0x10 ? "0" : ""));
    kartu.concat(String(mfrc522.uid.uidByte[i], HEX));
  }
  kartu.toUpperCase();
  Serial.println(kartu);

  String nama_kartu = "";
  Serial.println("🔍 Verifying card...");
  
  // STEP 1: Check Admin cards lokal (offline-first)
  if (verifyAccessLocal(kartu, nama_kartu)) {
    Serial.println("✅ Access GRANTED (ADMIN)");
    bener();
    buka();
    kirimPesan("✅ ACCESS GRANTED\nNama: " + nama_kartu + "\nKartu: " + kartu + "\nTipe: ADMIN\n" + getWaktuDanTanggal() + "\nHari: " + getHari());
  } 
  // STEP 2: Check scheduled cards dari server cache
  else if (verifyAccessServer(kartu, nama_kartu)) {
    Serial.println("✅ Access GRANTED (SCHEDULED)");
    bener();
    buka();
    kirimPesan("✅ ACCESS GRANTED\nNama: " + nama_kartu + "\nKartu: " + kartu + "\nTipe: SCHEDULED\n" + getWaktuDanTanggal() + "\nHari: " + getHari());
  } 
  // STEP 3: Not authorized
  else {
    Serial.println("❌ Access DENIED");
    salah();
    kirimPesan("❌ ACCESS DENIED\nKartu: " + kartu + "\nAlasan: Not authorized\n" + getWaktuDanTanggal() + "\nHari: " + getHari());
  }

  mfrc522.PICC_HaltA();
}

void bener() {
  digitalWrite(BUZ_PIN, HIGH); delay(100);
  digitalWrite(BUZ_PIN, LOW);  delay(100);
  digitalWrite(BUZ_PIN, HIGH); delay(100);
  digitalWrite(BUZ_PIN, LOW);  delay(100);
}

void salah() {
  digitalWrite(BUZ_PIN, HIGH); delay(500);
  digitalWrite(BUZ_PIN, LOW);  delay(100);
  digitalWrite(BUZ_PIN, HIGH); delay(100);
  digitalWrite(BUZ_PIN, LOW);  delay(100);
}

void buka() {
  Serial.println(">>> RELAY OPENING DOOR");
  Serial.print("Setting RLY_PIN ("); Serial.print(RLY_PIN); Serial.println(") to HIGH (ON)");
  
  digitalWrite(RLY_PIN, RELAY_ON);
  Serial.print("Relay state after ON: "); Serial.println(digitalRead(RLY_PIN));

  unsigned long start = millis();
  while (millis() - start < (unsigned long)(lama_buka_pintu * 1000)) {
    ArduinoOTA.handle();
  }

  Serial.println(">>> RELAY CLOSING DOOR");
  digitalWrite(RLY_PIN, RELAY_OFF);
  Serial.print("Relay state after OFF: "); Serial.println(digitalRead(RLY_PIN));
}

String urlEncode(String str) {
  String encoded = "";
  for (int i = 0; i < str.length(); i++) {
    char c = str.charAt(i);
    if ((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') ||
        (c >= 'a' && c <= 'z') || c == '-' || c == '_' || c == '.' || c == '~') {
      encoded += c;
    } else if (c == ' ') {
      encoded += "+";
    } else {
      encoded += "%";
      if ((unsigned char)c < 16) encoded += "0";
      encoded += String((unsigned char)c, HEX);
    }
  }
  return encoded;
}

void kirimPesan(String pesan) {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("WiFi not connected, skipping Telegram");
    return;
  }

  for (int attempt = 1; attempt <= 3; attempt++) {
    Serial.println("Telegram attempt " + String(attempt));
    
    HTTPClient http;
    http.setTimeout(10000);
    
    String url = "https://api.telegram.org/bot" + BOT_TOKEN + "/sendMessage";
    http.begin(url);
    http.addHeader("Content-Type", "application/x-www-form-urlencoded");

    String data = "chat_id=" + CHAT_ID + "&text=" + urlEncode(pesan);
    int httpResponseCode = http.POST(data);
    
    Serial.print("HTTP Response: "); Serial.println(httpResponseCode);
    http.end();

    if (httpResponseCode == 200) {
      Serial.println("Message sent!");
      return;
    }

    if (attempt < 3) delay(2000);
  }

  Serial.println("Telegram failed after 3 attempts");
}

String getWaktuDanTanggal() {
  time_t now = time(nullptr);
  struct tm* timeinfo = localtime(&now);
  char buffer[80];
  strftime(buffer, sizeof(buffer), "%d-%m-%Y %H:%M:%S", timeinfo);
  return String(buffer);
}

String getHari() {
  time_t now = time(nullptr);
  struct tm* timeinfo = localtime(&now);
  const char* hari[] = {"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"};
  return String(hari[timeinfo->tm_wday]);
}