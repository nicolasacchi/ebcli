package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseDate handles: "2024-01-15", "today", "yesterday", "-7d", "-30d"
func parseDate(input string) (time.Time, error) {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}

	today := truncateToDay(time.Now())

	switch input {
	case "today":
		return today, nil
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	}

	// Relative: -7d, -30d
	if strings.HasPrefix(input, "-") && strings.HasSuffix(input, "d") {
		daysStr := input[1 : len(input)-1]
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid relative date %q: %w", input, err)
		}
		return today.AddDate(0, 0, -days), nil
	}

	// Absolute: YYYY-MM-DD
	t, err := time.Parse("2006-01-02", input)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (expected YYYY-MM-DD, today, yesterday, or -Nd): %w", input, err)
	}
	return t, nil
}

// parseDateRange resolves --from/--to/--days into (from, to) dates.
// Default: last 30 days if nothing specified.
func parseDateRange(from, to, days string) (time.Time, time.Time, error) {
	today := truncateToDay(time.Now())

	// --days takes precedence as a shortcut
	if days != "" {
		d, err := strconv.Atoi(days)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --days value %q: %w", days, err)
		}
		return today.AddDate(0, 0, -d), today, nil
	}

	var fromDate, toDate time.Time
	var err error

	if from != "" {
		fromDate, err = parseDate(from)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --from: %w", err)
		}
	}

	if to != "" {
		toDate, err = parseDate(to)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --to: %w", err)
		}
	}

	// Defaults
	if fromDate.IsZero() && toDate.IsZero() {
		// No date flags: default to last 30 days
		return today.AddDate(0, 0, -30), today, nil
	}
	if fromDate.IsZero() {
		fromDate = toDate.AddDate(0, 0, -30)
	}
	if toDate.IsZero() {
		toDate = today
	}

	if fromDate.After(toDate) {
		return time.Time{}, time.Time{}, fmt.Errorf("--from (%s) is after --to (%s)", fromDate.Format("2006-01-02"), toDate.Format("2006-01-02"))
	}

	return fromDate, toDate, nil
}

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
