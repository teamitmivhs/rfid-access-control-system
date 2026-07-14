package handlers

import (
	"testing"
	"time"
)

func TestDeviceIsOnline(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.Local)
	if !deviceIsOnline(now.Add(-time.Minute).Format("2006-01-02 15:04:05"), now) {
		t.Fatal("heartbeat satu menit harus online")
	}
	if deviceIsOnline(now.Add(-3*time.Minute).Format("2006-01-02 15:04:05"), now) {
		t.Fatal("heartbeat tiga menit harus offline")
	}
	if deviceIsOnline("invalid", now) {
		t.Fatal("heartbeat invalid harus offline")
	}
}
