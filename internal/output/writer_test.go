package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPrinter_JSON_Pretty(t *testing.T) {
	var stdout bytes.Buffer
	p := NewPrinter(&stdout, &bytes.Buffer{}, ModePretty, false)

	data := map[string]string{"key": "value"}
	if err := p.JSON(data); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "  ") {
		t.Error("pretty mode should contain indentation")
	}
	if !json.Valid([]byte(strings.TrimSpace(got))) {
		t.Error("output should be valid JSON")
	}
}

func TestPrinter_JSON_Compact(t *testing.T) {
	var stdout bytes.Buffer
	p := NewPrinter(&stdout, &bytes.Buffer{}, ModeCompact, false)

	data := map[string]string{"key": "value"}
	if err := p.JSON(data); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if strings.Contains(got, "\n") {
		t.Error("compact mode should not contain newlines in JSON body")
	}
	if got != `{"key":"value"}` {
		t.Errorf("got %q, want %q", got, `{"key":"value"}`)
	}
}

func TestPrinter_Quiet(t *testing.T) {
	var stderr bytes.Buffer
	p := NewPrinter(&bytes.Buffer{}, &stderr, ModeCompact, true)

	p.Info("test info")
	p.Warn("test warn")
	p.Error("test error")

	if stderr.Len() > 0 {
		t.Errorf("quiet mode should suppress stderr, got: %q", stderr.String())
	}
}

func TestPrinter_StderrMessages(t *testing.T) {
	var stderr bytes.Buffer
	p := NewPrinter(&bytes.Buffer{}, &stderr, ModeCompact, false)

	p.Info("hello %s", "world")

	got := stderr.String()
	if !strings.Contains(got, "hello world") {
		t.Errorf("stderr should contain message, got: %q", got)
	}
}

func TestModeFromFlags(t *testing.T) {
	tests := []struct {
		pretty, compact, raw bool
		want                 Mode
	}{
		{false, false, false, ModeAuto},
		{true, false, false, ModePretty},
		{false, true, false, ModeCompact},
		{false, false, true, ModeRaw},
		{true, true, true, ModeRaw}, // raw takes priority
	}

	for _, tt := range tests {
		got := ModeFromFlags(tt.pretty, tt.compact, tt.raw)
		if got != tt.want {
			t.Errorf("ModeFromFlags(%v, %v, %v) = %d, want %d", tt.pretty, tt.compact, tt.raw, got, tt.want)
		}
	}
}
