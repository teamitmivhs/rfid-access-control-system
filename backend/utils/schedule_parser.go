package utils

import (
	"regexp"
	"strings"
)

type ScheduleOutput struct {
	Minggu []string `json:"Minggu"`
	Senin  []string `json:"Senin"`
	Selasa []string `json:"Selasa"`
	Rabu   []string `json:"Rabu"`
	Kamis  []string `json:"Kamis"`
	Jumat  []string `json:"Jumat"`
	Sabtu  []string `json:"Sabtu"`
}

var daysMap = map[string]string{
	"minggu":    "Minggu",
	"senin":     "Senin",
	"selasa":    "Selasa",
	"rabu":      "Rabu",
	"kamis":     "Kamis",
	"jumat":     "Jumat",
	"sabtu":     "Sabtu",
	"sunday":    "Minggu",
	"monday":    "Senin",
	"tuesday":   "Selasa",
	"wednesday": "Rabu",
	"thursday":  "Kamis",
	"friday":    "Jumat",
	"saturday":  "Sabtu",
}

var emptyDayValues = map[string]bool{
	"-":      true,
	"kosong": true,
	"libur":  true,
	"off":    true,
	"cuti":   true,
}

func ParseSchedule(input string) *ScheduleOutput {
	schedule := &ScheduleOutput{
		Minggu: []string{},
		Senin:  []string{},
		Selasa: []string{},
		Rabu:   []string{},
		Kamis:  []string{},
		Jumat:  []string{},
		Sabtu:  []string{},
	}

	if input == "" {
		return schedule
	}

	// Normalize input: lowercase, remove extra spaces
	input = strings.ToLower(input)
	input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")

	// Split by colon or dash to find day:names pairs
	lines := strings.FieldsFunc(input, func(r rune) bool {
		return r == '\n' || r == ';'
	})

	currentDay := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if line contains a day name
		dayFound := false
		for day := range daysMap {
			if strings.Contains(line, day) {
				// Extract the day name
				currentDay = daysMap[day]
				dayFound = true

				// Extract names after day (after : or space)
				parts := strings.FieldsFunc(line, func(r rune) bool {
					return r == ':' || r == '-' || r == '='
				})

				if len(parts) > 1 {
					namesStr := strings.Join(parts[1:], " ")
					names := parseNames(namesStr)
					appendNames(schedule, currentDay, names)
				}
				break
			}
		}

		// If no day found but currentDay is set, treat this as names for current day
		if !dayFound && currentDay != "" {
			names := parseNames(line)
			appendNames(schedule, currentDay, names)
		}
	}

	return schedule
}

func parseNames(namesStr string) []string {
	var names []string
	seen := make(map[string]bool)

	// Split by comma first
	parts := strings.Split(namesStr, ",")

	for _, part := range parts {
		// Clean up the part
		part = strings.TrimSpace(part)
		part = strings.Trim(part, ".")
		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		// Check if it's an empty day marker
		if _, isEmpty := emptyDayValues[strings.ToLower(part)]; isEmpty {
			continue
		}

		// Convert to uppercase and check if already seen
		upperPart := strings.ToUpper(part)
		if !seen[upperPart] {
			names = append(names, upperPart)
			seen[upperPart] = true
		}
	}

	return names
}

func appendNames(schedule *ScheduleOutput, day string, names []string) {
	if len(names) == 0 {
		return
	}

	switch day {
	case "Minggu":
		schedule.Minggu = append(schedule.Minggu, names...)
	case "Senin":
		schedule.Senin = append(schedule.Senin, names...)
	case "Selasa":
		schedule.Selasa = append(schedule.Selasa, names...)
	case "Rabu":
		schedule.Rabu = append(schedule.Rabu, names...)
	case "Kamis":
		schedule.Kamis = append(schedule.Kamis, names...)
	case "Jumat":
		schedule.Jumat = append(schedule.Jumat, names...)
	case "Sabtu":
		schedule.Sabtu = append(schedule.Sabtu, names...)
	}

	// Remove duplicates within the day
	removeScheduleDuplicates(schedule, day)
}

func removeScheduleDuplicates(schedule *ScheduleOutput, day string) {
	var daySlice *[]string

	switch day {
	case "Minggu":
		daySlice = &schedule.Minggu
	case "Senin":
		daySlice = &schedule.Senin
	case "Selasa":
		daySlice = &schedule.Selasa
	case "Rabu":
		daySlice = &schedule.Rabu
	case "Kamis":
		daySlice = &schedule.Kamis
	case "Jumat":
		daySlice = &schedule.Jumat
	case "Sabtu":
		daySlice = &schedule.Sabtu
	}

	if daySlice == nil {
		return
	}

	seen := make(map[string]bool)
	var unique []string

	for _, name := range *daySlice {
		if !seen[name] {
			unique = append(unique, name)
			seen[name] = true
		}
	}

	*daySlice = unique
}
