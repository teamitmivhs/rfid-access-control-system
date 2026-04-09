#include <SPI.h>
#include <MFRC522.h>
#include <WiFi.h>
#include <HTTPClient.h>
#include <ArduinoOTA.h>  // === OTA ===
#include <time.h>

void bener();
void salah();
void buka();
void kirimPesan(String pesan);
String getWaktuDanTanggal();
String getHari();

const char* ssid = "TEAM IT";
const char* password = "F@zzw0rd2026";

String BOT_TOKEN = "8683423891:AAFTBmo3owh5sA0MGPgvX5IpZv3lI7iFYFc";
String CHAT_ID = "-5059128316";

#define LRM_PIN 26
#define BUZ_PIN 13
#define RLY_PIN 14
#define RST_PIN 22
#define SS_PIN 21
#define PB_PIN 27

int jeda2 = 200;
int jeda1 = jeda2;
bool logika1 = false;
bool logika2 = false;
bool logika3 = false;
bool logika4 = false;
float lama_buka_pintu = 1.5;
unsigned long waktu = 0;

const int jumlah_kartu = 33;

String daftarUID[jumlah_kartu] = {
  "F3B812BA",
  "43358A01",
  "132531BD",
  "51D21103",
  "61960703",
  "43336D01",
  "F3B511BA",
  "53F210BA",
  "333E2101",
  "D3C80ABA",
  "76763FBC",
  "86C022BC",
  "B626FBBB",
  "1684FCBB",
  "D67EFDBB",
  "86EEFCBB",
  "43BD0A03",
  "12988D8B",
  "46D722BC",
  "B6C7FEBB",
  "03663C0E",
  "068343BC",
  "068343BC",
  "9B7EB350",
  "A6D6FCBB",
  "A3CD0903",
  "187E8C8B",
  "13BC6B2D",
  "E33B782A",
  "9CA42849",
  "938934FF",
};

String daftarNama[jumlah_kartu] = {
  "F3B812BA",
  "43358A01",
  "132531BD",
  "51D21103",
  "61960703",
  "43336D01",
  "F3B511BA",
  "53F210BA",
  "333E2101",
  "D3C80ABA",
  "76763FBC",
  "86C022BC",
  "B626FBBB",
  "1684FCBB",
  "D67EFDBB",
  "86EEFCBB",
  "43BD0A03",
  "GANI",
  "46D722BC",
  "FIKRI",
  "03663C0E",
  "068343BC",
  "068343BC",
  "9B7EB350",
  "A6D6FCBB",
  "A3CD0903",
  "GHONI",
  "13BC6B2D",
  "E33B782A",
  "9CA42849",
  "ALVARO",
};

MFRC522 mfrc522(SS_PIN, RST_PIN);

void setup() {
  Serial.begin(9600);
  while (!Serial)
    ;

  SPI.begin();
  mfrc522.PCD_Init();

  pinMode(BUZ_PIN, OUTPUT);
  pinMode(RLY_PIN, OUTPUT);
  pinMode(LRM_PIN, INPUT_PULLUP);
  pinMode(PB_PIN, INPUT_PULLUP);

  digitalWrite(RLY_PIN, LOW);
  digitalWrite(BUZ_PIN, LOW);

  WiFi.begin(ssid, password);
  Serial.print("Connecting to WiFi");
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println("Connected!");
  
  // == Intgerasi waktu via NTP ==
  configTime(7 * 3600, 0, "pool.ntp.org", "time.nist.gov");
  Serial.println("Waiting for NTP time sync...");
  time_t now = time(nullptr);
  while (now < 24 * 3600) {
    delay(500);
    Serial.print(".");
    now = time(nullptr);
  }
  Serial.println();
  Serial.println("Time synced!");
  
  kirimPesan("Door LOCK wifi connect successful\n" + getWaktuDanTanggal() + "\n" + getHari());

  // === OTA ===
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
    if (error == OTA_AUTH_ERROR) Serial.println("Auth Failed");
    else if (error == OTA_BEGIN_ERROR) Serial.println("Begin Failed");
    else if (error == OTA_CONNECT_ERROR) Serial.println("Connect Failed");
    else if (error == OTA_RECEIVE_ERROR) Serial.println("Receive Failed");
    else if (error == OTA_END_ERROR) Serial.println("End Failed");
  });

  ArduinoOTA.begin();
  // === OTA END ===
}

void loop() {
  ArduinoOTA.handle();  // === OTA handler wajib di loop ===

  if (digitalRead(PB_PIN) == LOW) logika2 = true;

  while (logika2) {
    if (digitalRead(PB_PIN) == HIGH) {
      logika2 = false;
      buka();
    }
  }

  if (!logika1 && !logika2 && digitalRead(LRM_PIN) == HIGH) {
    if (waktu == 0) waktu = millis();
    else if (millis() - waktu >= 11000 && !logika3) logika3 = true;
  }

  if (logika3) {
    digitalWrite(BUZ_PIN, HIGH);
    delay(jeda1);
    digitalWrite(BUZ_PIN, LOW);
    delay(jeda1);
    if (jeda1 >= 70) jeda1 -= 1;
  }

  if (digitalRead(LRM_PIN) == LOW) {
    waktu = 0;
    if (logika3) {
      logika3 = false;
      jeda1 = jeda2;
    }
  }

  if (!mfrc522.PICC_IsNewCardPresent() || !mfrc522.PICC_ReadCardSerial()) return;

  String kartu = "";
  Serial.print(F("Card UID:"));
  for (byte i = 0; i < mfrc522.uid.size; i++) {
    kartu.concat(String(mfrc522.uid.uidByte[i] < 0x10 ? "0" : ""));
    kartu.concat(String(mfrc522.uid.uidByte[i], HEX));
  }
  kartu.toUpperCase();
  Serial.println(kartu);

  String nama = "";
  for (int i = 0; i < jumlah_kartu; i++) {
    if (kartu == daftarUID[i]) {
      logika1 = true;
      nama = daftarNama[i];
      break;
    }
  }

  if (logika1) {
    logika1 = false;
    Serial.println("access granted");
    bener();
    buka();
    kirimPesan("Nama: " + nama + "\nKartu: " + kartu + "\nStatus: Access Granted\n" + getWaktuDanTanggal() + "\nHari: " + getHari());
  } else {
    Serial.println("access denied");
    salah();
    kirimPesan("Kartu: " + kartu + "\nStatus: Access Denied\n" + getWaktuDanTanggal() + "\nHari: " + getHari());
  }

  mfrc522.PICC_HaltA();
}

void bener() {
  digitalWrite(BUZ_PIN, HIGH);
  delay(100);
  digitalWrite(BUZ_PIN, LOW);
  delay(100);
  digitalWrite(BUZ_PIN, HIGH);
  delay(100);
  digitalWrite(BUZ_PIN, LOW);
  delay(100);
}

void salah() {
  digitalWrite(BUZ_PIN, HIGH);
  delay(500);
  digitalWrite(BUZ_PIN, LOW);
  delay(100);
  digitalWrite(BUZ_PIN, HIGH);
  delay(100);
  digitalWrite(BUZ_PIN, LOW);
  delay(100);
}

void buka() {
  digitalWrite(RLY_PIN, HIGH);
  delay(lama_buka_pintu * 1000);
  digitalWrite(RLY_PIN, LOW);
}

void kirimPesan(String pesan) {
  if (WiFi.status() == WL_CONNECTED) {
    HTTPClient http;
    String url = "https://api.telegram.org/bot" + BOT_TOKEN + "/sendMessage";
    http.begin(url);
    http.addHeader("Content-Type", "application/x-www-form-urlencoded");
    String data = "chat_id=" + CHAT_ID + "&text=" + pesan;
    int httpResponseCode = http.POST(data);
    http.end();
  } else {
    Serial.println("WiFi not connected");
  }
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
