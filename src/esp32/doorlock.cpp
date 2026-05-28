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
void checkAndExecutePendingSync();
void confirmSyncCompleted();

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

//kartu admin
const int jumlah_kartu = 6;
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

//kartu dari server
const int MAX_SERVER_CARDS = 100;
String serverCardUID[MAX_SERVER_CARDS];
String serverCardNama[MAX_SERVER_CARDS];
int serverCardCount = 0;
unsigned long lastSyncTime = 0;
unsigned long lastSyncStatusCheckTime = 0;
int syncFailCount = 0;
const int MAX_FAIL_BEFORE_NOTIF = 3;
const unsigned long SYNC_INTERVAL = 3600000;           // Sync scheduled: 1 jam
const unsigned long SYNC_STATUS_CHECK_INTERVAL = 10000; // Check pending sync: 10 detik

MFRC522 mfrc522(SS_PIN, RST_PIN);

// Verifikasi kartu secara lokal (Admin)
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

// Verifikasi kartu dari server
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

// === UPDATED: Sync dari server (simplified, no TCP test) ===
void syncCardsFromServer() {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("⚠️  WiFi not connected, skipping sync");
    return;
  }

  Serial.println("\n🔄 Syncing scheduled cards from server...");

  HTTPClient http;
  WiFiClient client;
  
  http.setTimeout(15000);
  http.setConnectTimeout(10000);
  
  String url = "http://192.168.107.37:8081/api/cards/scheduled-today";
  Serial.println("URL: " + url);
  
  if (!http.begin(client, url)) {
    Serial.println("❌ HTTP begin failed");
    syncFailCount++;
    if (syncFailCount >= MAX_FAIL_BEFORE_NOTIF) {
      kirimPesan("⚠️ SYNC GAGAL\nHTTP begin failed\nGagal: " + String(syncFailCount) + "x\n" + getWaktuDanTanggal());
      syncFailCount = 0;
    }
    return;
  }

  int httpCode = http.GET();
  Serial.println("HTTP Code: " + String(httpCode));
  
  if (httpCode != 200) {
    Serial.println("❌ HTTP Error: " + String(httpCode));
    http.end();
    syncFailCount++;
    if (syncFailCount >= MAX_FAIL_BEFORE_NOTIF) {
      kirimPesan("⚠️ SYNC GAGAL - HTTP " + String(httpCode) + "\nGagal: " + String(syncFailCount) + "x\n" + getWaktuDanTanggal());
      syncFailCount = 0;
    }
    return;
  }

  String response = http.getString();
  Serial.println("Response: " + response);
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
    Serial.println("  ✓ Added: " + uid + " (" + nama + ")");
    count++;
  }

  serverCardCount = count;
  syncFailCount = 0;

  Serial.println("✅ Sync complete! " + String(serverCardCount) + " cards loaded for " + doc["hari"].as<String>());
}

// === NEW: Check apakah ada pending sync dari Telegram ===
void checkAndExecutePendingSync() {
  if (WiFi.status() != WL_CONNECTED) {
    return; // Skip kalau WiFi down
  }

  HTTPClient http;
  WiFiClient client;
  
  String url = "http://192.168.107.37:8081/api/sync-status";
  
  if (!http.begin(client, url)) {
    Serial.println("❌ Failed to check sync status");
    return;
  }

  int httpCode = http.GET();
  
  if (httpCode != 200) {
    Serial.println("⚠️  Sync status check failed (HTTP " + String(httpCode) + ")");
    http.end();
    return;
  }

  String response = http.getString();
  http.end();

  DynamicJsonDocument doc(512);
  if (deserializeJson(doc, response)) {
    Serial.println("❌ JSON parse error");
    return;
  }

  bool shouldSync = doc["should_sync"].as<bool>();
  
  if (shouldSync) {
    Serial.println("\n📲 PENDING SYNC DETECTED FROM TELEGRAM!");
    Serial.println("Executing sync now...");
    
    syncCardsFromServer();
    
    // Confirm sync ke server
    delay(1000);
    confirmSyncCompleted();
  }
}

// === NEW: Confirm ke server bahwa sync sudah selesai ===
void confirmSyncCompleted() {
  if (WiFi.status() != WL_CONNECTED) {
    return;
  }

  HTTPClient http;
  WiFiClient client;
  
  String url = "http://192.168.107.37:8081/api/confirm-sync";
  
  if (!http.begin(client, url)) {
    Serial.println("❌ Failed to confirm sync");
    return;
  }

  int httpCode = http.POST("");
  Serial.println("Confirm sync response: HTTP " + String(httpCode));
  http.end();
}

void setup() {
  Serial.begin(9600);
  delay(1000);
  Serial.println("\n\n=== DOOR LOCK STARTING ===");
  
  pinMode(RLY_PIN, OUTPUT);
  Serial.println("RLY_PIN: " + String(RLY_PIN));
  digitalWrite(RLY_PIN, RELAY_OFF);
  delay(500);

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
    Serial.println("ERROR: RFID tidak terdeteksi!");
  } else {
    Serial.println("RFID OK");
  }

  IPAddress local_IP(192, 168, 107, 100);
  IPAddress gateway(192, 168, 96, 1);
  IPAddress subnet(255, 255, 240, 0);
  IPAddress dns(8, 8, 8, 8);
  WiFi.config(local_IP, gateway, subnet, dns);

  WiFi.begin(ssid, password);
  Serial.print("Connecting to WiFi");
  int wifiTimeout = 0;
  while (WiFi.status() != WL_CONNECTED && wifiTimeout < 20) {
    delay(500);
    Serial.print(".");
    wifiTimeout++;
  }
  Serial.println("\nConnected!");
  Serial.println(WiFi.localIP());

  configTime(7 * 3600, 0, "pool.ntp.org", "time.nist.gov");
  Serial.println("Waiting for NTP time sync...");
  time_t now = time(nullptr);
  int ntpTimeout = 0;
  while (now < 24 * 3600 && ntpTimeout < 10) {
    delay(500);
    Serial.print(".");
    now = time(nullptr);
    ntpTimeout++;
  }
  Serial.println("\nTime synced!");

  Serial.println("\n📥 Syncing scheduled cards from server...");
  syncCardsFromServer();

  kirimPesan("Door LOCK online\n" + getWaktuDanTanggal() + "\n" + getHari());

  ArduinoOTA.setHostname("DoorLock-ESP32");
  ArduinoOTA.begin();
}

void loop() {
  ArduinoOTA.handle();

  // === SCHEDULED SYNC (1 jam sekali) ===
  if (WiFi.status() == WL_CONNECTED) {
    if (lastSyncTime == 0 || (millis() - lastSyncTime >= SYNC_INTERVAL)) {
      lastSyncTime = millis();
      syncCardsFromServer();
    }
  }

  // === PENDING SYNC CHECK (10 detik sekali) ===
  if (WiFi.status() == WL_CONNECTED) {
    if (lastSyncStatusCheckTime == 0 || (millis() - lastSyncStatusCheckTime >= SYNC_STATUS_CHECK_INTERVAL)) {
      lastSyncStatusCheckTime = millis();
      checkAndExecutePendingSync();  // ← Cek apakah ada /sync dari Telegram
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
  
  if (verifyAccessLocal(kartu, nama_kartu)) {
    Serial.println("✅ Access GRANTED (ADMIN)");
    bener();
    buka();
    kirimPesan("✅ ACCESS GRANTED\nNama: " + nama_kartu + "\nKartu: " + kartu + "\nTipe: ADMIN\n" + getWaktuDanTanggal() + "\nHari: " + getHari());
  } 
  else if (verifyAccessServer(kartu, nama_kartu)) {
    Serial.println("✅ Access GRANTED (SCHEDULED)");
    bener();
    buka();
    kirimPesan("✅ ACCESS GRANTED\nNama: " + nama_kartu + "\nKartu: " + kartu + "\nTipe: SCHEDULED\n" + getWaktuDanTanggal() + "\nHari: " + getHari());
  } 
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
  digitalWrite(RLY_PIN, RELAY_ON);
  Serial.println("Relay ON");

  unsigned long start = millis();
  while (millis() - start < (unsigned long)(lama_buka_pintu * 1000)) {
    ArduinoOTA.handle();
  }

  Serial.println(">>> RELAY CLOSING DOOR");
  digitalWrite(RLY_PIN, RELAY_OFF);
  Serial.println("Relay OFF");
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
