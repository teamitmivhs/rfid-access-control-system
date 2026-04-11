package utils

import (
	"testing"
)

func TestParseSchedule(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string][]string
	}{
		{
			name:  "Simple format with colons",
			input: "Senin: ALVARO, FIKRI\nSelasa: GANI\nRabu: -",
			expected: map[string][]string{
				"Minggu": {},
				"Senin":  {"ALVARO", "FIKRI"},
				"Selasa": {"GANI"},
				"Rabu":   {},
				"Kamis":  {},
				"Jumat":  {},
				"Sabtu":  {},
			},
		},
		{
			name:  "Lowercase names converted to uppercase",
			input: "Senin: alvaro, fikri",
			expected: map[string][]string{
				"Minggu": {},
				"Senin":  {"ALVARO", "FIKRI"},
				"Selasa": {},
				"Rabu":   {},
				"Kamis":  {},
				"Jumat":  {},
				"Sabtu":  {},
			},
		},
		{
			name:  "Empty day with kosong marker",
			input: "Senin: kosong\nSelasa: ALVARO",
			expected: map[string][]string{
				"Minggu": {},
				"Senin":  {},
				"Selasa": {"ALVARO"},
				"Rabu":   {},
				"Kamis":  {},
				"Jumat":  {},
				"Sabtu":  {},
			},
		},
		{
			name:  "Duplicate names in same day",
			input: "Senin: ALVARO, fikri, alvaro",
			expected: map[string][]string{
				"Minggu": {},
				"Senin":  {"ALVARO", "FIKRI"},
				"Selasa": {},
				"Rabu":   {},
				"Kamis":  {},
				"Jumat":  {},
				"Sabtu":  {},
			},
		},
		{
			name:  "Extra spaces and punctuation",
			input: "Senin:  ALVARO  ,  FIKRI  .\nSelasa: GANI, GHONI.",
			expected: map[string][]string{
				"Minggu": {},
				"Senin":  {"ALVARO", "FIKRI"},
				"Selasa": {"GANI", "GHONI"},
				"Rabu":   {},
				"Kamis":  {},
				"Jumat":  {},
				"Sabtu":  {},
			},
		},
		{
			name:  "English day names",
			input: "Monday: ALVARO\nTuesday: FIKRI",
			expected: map[string][]string{
				"Minggu": {},
				"Senin":  {"ALVARO"},
				"Selasa": {"FIKRI"},
				"Rabu":   {},
				"Kamis":  {},
				"Jumat":  {},
				"Sabtu":  {},
			},
		},
		{
			name:  "Empty input",
			input: "",
			expected: map[string][]string{
				"Minggu": {},
				"Senin":  {},
				"Selasa": {},
				"Rabu":   {},
				"Kamis":  {},
				"Jumat":  {},
				"Sabtu":  {},
			},
		},
		{
			name:  "Dash marker for empty day",
			input: "Senin: -\nSelasa: ALVARO",
			expected: map[string][]string{
				"Minggu": {},
				"Senin":  {},
				"Selasa": {"ALVARO"},
				"Rabu":   {},
				"Kamis":  {},
				"Jumat":  {},
				"Sabtu":  {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSchedule(tt.input)

			checkScheduleDay(t, "Minggu", result.Minggu, tt.expected["Minggu"])
			checkScheduleDay(t, "Senin", result.Senin, tt.expected["Senin"])
			checkScheduleDay(t, "Selasa", result.Selasa, tt.expected["Selasa"])
			checkScheduleDay(t, "Rabu", result.Rabu, tt.expected["Rabu"])
			checkScheduleDay(t, "Kamis", result.Kamis, tt.expected["Kamis"])
			checkScheduleDay(t, "Jumat", result.Jumat, tt.expected["Jumat"])
			checkScheduleDay(t, "Sabtu", result.Sabtu, tt.expected["Sabtu"])
		})
	}
}

func checkScheduleDay(t *testing.T, day string, got []string, expected []string) {
	if len(got) != len(expected) {
		t.Errorf("%s: expected %d names, got %d. Expected: %v, Got: %v", day, len(expected), len(got), expected, got)
		return
	}

	for i, name := range got {
		if name != expected[i] {
			t.Errorf("%s[%d]: expected %s, got %s", day, i, expected[i], name)
		}
	}
}
