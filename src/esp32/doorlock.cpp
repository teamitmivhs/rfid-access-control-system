#include <SPI.h>
#include <MFRC522.h>
#include <WiFi.h>
#include <HTTPClient.h>
#include <ArduinoOTA.h>
#include <time.h>

void bener();
void salah();
void buka();
void kirimPesan(String pesan);
String getWaktuDanTanggal();
String getHari();
String urlEncode(String str);
bool verifyAccessLocal(String uid, String &nama);

const char* ssid     = "TEAM IT MIVHS";
const char* password = "1TM1TR4101101MIVHS2025PASTIBISA2026SELALULANCAR2027GENERASI2028";

String BOT_TOKEN = "8683423891:AAFTBmo3owh5sA0MGPgvX5IpZv3lI7iFYFc";
String CHAT_ID   = "-1003302843795";

#define LRM_PIN        26
#define BUZ_PIN        13
#define RLY_PIN        4
#define RST_PIN        22
#define SS_PIN         21
#define MANUAL_BTN_PIN 15

#define SPI_SCK  18
#define SPI_MISO 19
#define SPI_MOSI 23
#define RELAY_ON  HIGH
#define RELAY_OFF LOW

int   jeda2           = 200;
int   jeda1           = jeda2;
bool  logika1         = false;
bool  logika3         = false;
float lama_buka_pintu = 2.0;
unsigned long waktu   = 0;

//manual button variables
bool manual_btn_state             = true;
bool manual_btn_prev              = true;
unsigned long manual_btn_debounce = 0;
unsigned long relay_active_time   = 0;
bool relay_is_active              = false;
const unsigned long DEBOUNCE_DELAY = 50;
const unsigned long RELAY_HOLD_TIME = 2000;

const int jumlah_kartu = 3; // Jumlah kartu yang terdaftar

// Database kartu akses lokal
const String daftarUID[3] = {
  "938934FF",   // Kartu 1
  "43358A01",   // Kartu 2 
  "A1B2C3D4"    // Kartu 3
};

const String daftarNama[3] = {
  "ALVARO (ADMIN)",
  "Kartu User 1",
  "Kartu User 2"
};

MFRC522 mfrc522(SS_PIN, RST_PIN);

// Verifikasi kartu secara lokal (tanpa backend)
bool verifyAccessLocal(String uid, String &nama) {
  for (int i = 0; i < jumlah_kartu; i++) {
    if (uid == daftarUID[i]) {
      nama = daftarNama[i];
      Serial.println("Kartu cocok: " + nama);
      return true;  // Akses diberikan
    }
  }
  Serial.println("Kartu tidak terdaftar");
  return false;  // Kartu tidak dikenal
}

void setup() {
  Serial.begin(9600);
  delay(1000);
  Serial.println("\n\n=== DOOR LOCK STARTING ===");
  
  //matiin relay sebelom mulai, jadi maglock kekunci abis restart
  pinMode(RLY_PIN, OUTPUT);
  
  Serial.println("\n*** RELAY DIAGNOSTIC TEST ***");
  Serial.print("RLY_PIN: ");
  Serial.println(RLY_PIN);
  Serial.print("RELAY_ON (HIGH): ");
  Serial.println(RELAY_ON);
  Serial.print("RELAY_OFF (LOW): ");
  Serial.println(RELAY_OFF);
  
  // Set relay to OFF state (NC - door locked)
  digitalWrite(RLY_PIN, RELAY_OFF);
  delay(500);
  Serial.print("Relay state after RELAY_OFF: ");
  Serial.println(digitalRead(RLY_PIN));
  
  // Test toggle relay 3x untuk diagnosa
  Serial.println("\nToggle relay 3x untuk test GPIO4...");
  for (int i = 0; i < 3; i++) {
    Serial.print("Toggle ");
    Serial.print(i + 1);
    Serial.print(" - ON: ");
    digitalWrite(RLY_PIN, RELAY_ON);
    delay(300);
    Serial.println(digitalRead(RLY_PIN));
    
    Serial.print("Toggle ");
    Serial.print(i + 1);
    Serial.print(" - OFF: ");
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

  //verifikasi RFID
  byte ver = mfrc522.PCD_ReadRegister(MFRC522::VersionReg);
  Serial.print("MFRC522 version: 0x");
  Serial.println(ver, HEX);
  if (ver == 0x00 || ver == 0xFF) {
    Serial.println("ERROR: RFID tidak terdeteksi! Cek wiring SPI.");
  } else {
    Serial.println("RFID OK");
  }

  // Verify relay is OFF before continuing
  Serial.println("\n*** Relay Verification ***");
  for (int i = 0; i < 3; i++) {
    Serial.print("Check ");
    Serial.print(i + 1);
    Serial.print(": Relay pin state = ");
    Serial.println(digitalRead(RLY_PIN));
    if (digitalRead(RLY_PIN) != LOW) {
      Serial.println("WARNING: Relay not OFF! Trying to set OFF again...");
      digitalWrite(RLY_PIN, RELAY_OFF);
      delay(500);
    }
  }
  Serial.println("*** Relay OK ***\n");

  WiFi.begin(ssid, password);
  Serial.print("Connecting to WiFi");
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println("\nConnected!");
  Serial.println(WiFi.localIP());

  //sinkronisasi waktu via NTP
  configTime(7 * 3600, 0, "pool.ntp.org", "time.nist.gov");
  Serial.println("Waiting for NTP time sync...");
  time_t now = time(nullptr);
  while (now < 24 * 3600) {
    delay(500);
    Serial.print(".");
    now = time(nullptr);
  }
  Serial.println("\nTime synced!");

  kirimPesan("Door LOCK online\n" + getWaktuDanTanggal() + "\n" + getHari());

  //ota
  ArduinoOTA.setHostname("DoorLock-ESP32");
  ArduinoOTA.onStart([]() {
    Serial.println("Start OTA update");
  });
  ArduinoOTA.onEnd([]() {
    Serial.println("\nEnd OTA");
  });
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
  //ota end
}

void loop() {
  ArduinoOTA.handle();

  //manual button logic
  manual_btn_state = digitalRead(MANUAL_BTN_PIN);

  if (manual_btn_state == LOW && manual_btn_prev == HIGH) {
    manual_btn_debounce = millis();
  }

  if (manual_btn_state == LOW && (millis() - manual_btn_debounce >= DEBOUNCE_DELAY)) {
    if (manual_btn_prev == HIGH) {
      Serial.println("Tombol ditekan - Membuka pintu");
      digitalWrite(BUZ_PIN, HIGH);
      delay(100);
      digitalWrite(BUZ_PIN, LOW);

      digitalWrite(RLY_PIN, RELAY_ON);
      relay_is_active   = true;
      relay_active_time = millis();
      Serial.println("Relay ON - Pintu TERBUKA");
    }
  }
  manual_btn_prev = manual_btn_state;

  //auto-lock setelah 2 detik
  if (relay_is_active && (millis() - relay_active_time >= RELAY_HOLD_TIME)) {
    digitalWrite(RLY_PIN, RELAY_OFF);
    relay_is_active = false;
    Serial.println("Relay OFF - Pintu TERKUNCI (auto-lock)");
    digitalWrite(BUZ_PIN, HIGH); delay(50);
    digitalWrite(BUZ_PIN, LOW);  delay(50);
    digitalWrite(BUZ_PIN, HIGH); delay(50);
    digitalWrite(BUZ_PIN, LOW);
  }
  //end manual button logic

  //lrm logic
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
  //end lrm logic

  if (!mfrc522.PICC_IsNewCardPresent() || !mfrc522.PICC_ReadCardSerial()) return;

  String kartu = "";
  Serial.print(F("Card UID: "));
  for (byte i = 0; i < mfrc522.uid.size; i++) {
    kartu.concat(String(mfrc522.uid.uidByte[i] < 0x10 ? "0" : ""));
    kartu.concat(String(mfrc522.uid.uidByte[i], HEX));
  }
  kartu.toUpperCase();
  Serial.println(kartu);

  // Verifikasi kartu secara lokal
  String nama_kartu = "";
  Serial.println("Verifying card locally...");
  
  if (verifyAccessLocal(kartu, nama_kartu)) {
    Serial.println("Access GRANTED: " + nama_kartu);
    bener();
    buka();
    kirimPesan("Nama: " + nama_kartu + "\nKartu: " + kartu + "\nStatus: ACCESS GRANTED\n" + getWaktuDanTanggal() + "\nHari: " + getHari());
  } else {
    Serial.println("Access DENIED: Kartu tidak terdaftar");
    salah();
    kirimPesan("Kartu: " + kartu + "\nStatus: CARD NOT REGISTERED\n" + getWaktuDanTanggal() + "\nHari: " + getHari());
  }

  mfrc522.PICC_HaltA();
}

//buzzer 2x pendek = akses diterima
void bener() {
  digitalWrite(BUZ_PIN, HIGH); delay(100);
  digitalWrite(BUZ_PIN, LOW);  delay(100);
  digitalWrite(BUZ_PIN, HIGH); delay(100);
  digitalWrite(BUZ_PIN, LOW);  delay(100);
}

//buzzer 1x panjang + 1x pendek = akses ditolak
void salah() {
  digitalWrite(BUZ_PIN, HIGH); delay(500);
  digitalWrite(BUZ_PIN, LOW);  delay(100);
  digitalWrite(BUZ_PIN, HIGH); delay(100);
  digitalWrite(BUZ_PIN, LOW);  delay(100);
}

//buka pintu: aktifkan relay selama lama_buka_pintu detik lalu kunci lagi
void buka() {
  Serial.println(">>> RELAY OPENING DOOR");
  Serial.print("Setting RLY_PIN (");
  Serial.print(RLY_PIN);
  Serial.println(") to HIGH (ON)");
  
  digitalWrite(RLY_PIN, RELAY_ON);   //high → relay aktif → maglock mati → TERBUKA
  Serial.print("Relay state after ON: ");
  Serial.println(digitalRead(RLY_PIN));

  unsigned long start = millis();
  while (millis() - start < (unsigned long)(lama_buka_pintu * 1000)) {
    ArduinoOTA.handle();
  }

  Serial.println(">>> RELAY CLOSING DOOR");
  digitalWrite(RLY_PIN, RELAY_OFF);  //low → relay mati → maglock nyala → TERKUNCI
  Serial.print("Relay state after OFF: ");
  Serial.println(digitalRead(RLY_PIN));
}

String urlEncode(String str) {
  String encoded = "";
  for (int i = 0; i < str.length(); i++) {
    char c = str.charAt(i);
    if ((c >= '0' && c <= '9') ||
        (c >= 'A' && c <= 'Z') ||
        (c >= 'a' && c <= 'z') ||
        c == '-' || c == '_' || c == '.' || c == '~') {
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

  // Retry maksimal 3x
  for (int attempt = 1; attempt <= 3; attempt++) {
    Serial.println("Telegram attempt " + String(attempt));
    
    HTTPClient http;
    http.setTimeout(10000);  // timeout 10 detik
    
    String url = "https://api.telegram.org/bot" + BOT_TOKEN + "/sendMessage";
    http.begin(url);
    http.addHeader("Content-Type", "application/x-www-form-urlencoded");

    String data = "chat_id=" + CHAT_ID + "&text=" + urlEncode(pesan);
    int httpResponseCode = http.POST(data);
    
    Serial.print("HTTP Response: ");
    Serial.println(httpResponseCode);

    http.end();

    if (httpResponseCode == 200) {
      Serial.println("Message sent!");
      return;  // sukses, keluar
    }

    if (attempt < 3) delay(2000);  // tunggu 2 detik sebelum retry
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