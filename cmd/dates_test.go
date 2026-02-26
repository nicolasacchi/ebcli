package cmd

import (
	"testing"
	"time"
)

func TestParseDate(t *testing.T) {
	now := truncateToDay(time.Now())

	tests := []struct {
		input   string
		want    time.Time
		wantErr bool
	}{
		{"2024-01-15", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC), false},
		{"today", now, false},
		{"yesterday", now.AddDate(0, 0, -1), false},
		{"-7d", now.AddDate(0, 0, -7), false},
		{"-30d", now.AddDate(0, 0, -30), false},
		{"-0d", now, false},
		{"", time.Time{}, true},
		{"invalid", time.Time{}, true},
		{"-xd", time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("parseDate(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDateRange_Defaults(t *testing.T) {
	from, to, err := parseDateRange("", "", "")
	if err != nil {
		t.Fatalf("parseDateRange defaults: %v", err)
	}

	today := truncateToDay(time.Now())
	expectedFrom := today.AddDate(0, 0, -30)

	if !from.Equal(expectedFrom) {
		t.Errorf("from = %v, want %v", from, expectedFrom)
	}
	if !to.Equal(today) {
		t.Errorf("to = %v, want %v", to, today)
	}
}

func TestParseDateRange_DaysFlag(t *testing.T) {
	from, to, err := parseDateRange("", "", "7")
	if err != nil {
		t.Fatalf("parseDateRange --days 7: %v", err)
	}

	today := truncateToDay(time.Now())
	expectedFrom := today.AddDate(0, 0, -7)

	if !from.Equal(expectedFrom) {
		t.Errorf("from = %v, want %v", from, expectedFrom)
	}
	if !to.Equal(today) {
		t.Errorf("to = %v, want %v", to, today)
	}
}

func TestParseDateRange_FromTo(t *testing.T) {
	from, to, err := parseDateRange("2024-01-01", "2024-01-31", "")
	if err != nil {
		t.Fatalf("parseDateRange from/to: %v", err)
	}

	if from != time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) {
		t.Errorf("from = %v", from)
	}
	if to != time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC) {
		t.Errorf("to = %v", to)
	}
}

func TestParseDateRange_FromAfterTo(t *testing.T) {
	_, _, err := parseDateRange("2024-02-01", "2024-01-01", "")
	if err == nil {
		t.Fatal("expected error when from > to")
	}
}
