package handler

import (
	"testing"
	"time"

	unirunapi "autorun-go/unirunapi"
)

func TestParseClubEventTime(t *testing.T) {
	loc := time.FixedZone("test", 8*60*60)
	tests := []struct {
		name  string
		raw   string
		wantH int
		wantM int
	}{
		{name: "hour minute", raw: "18:30", wantH: 18, wantM: 30},
		{name: "full datetime", raw: "2026-05-13 19:05:00", wantH: 19, wantM: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseClubEventTime("2026-05-13", tt.raw, loc)
			if !ok {
				t.Fatalf("parseClubEventTime(%q) returned false", tt.raw)
			}
			if got.Year() != 2026 || got.Month() != time.May || got.Day() != 13 {
				t.Fatalf("unexpected date: %s", got)
			}
			if got.Hour() != tt.wantH || got.Minute() != tt.wantM {
				t.Fatalf("unexpected clock: %s", got)
			}
		})
	}
}

func TestDueClubProbes(t *testing.T) {
	now := time.Date(2026, time.May, 13, 18, 51, 0, 0, time.Local)
	activities := []unirunapi.ClubInfo{
		{ClubActivityID: 123, StartTime: "19:00", EndTime: "20:00"},
	}

	probes := dueClubProbes(now, "2026-05-13", activities, clubScheduleEntry{})
	if len(probes) != 1 {
		t.Fatalf("expected one probe, got %d", len(probes))
	}
	if probes[0].SignType != "1" {
		t.Fatalf("expected sign-in probe, got %q", probes[0].SignType)
	}

	done := clubScheduleEntry{LastSignInKey: clubProbeKey("2026-05-13", 123, "1")}
	probes = dueClubProbes(now, "2026-05-13", activities, done)
	if len(probes) != 0 {
		t.Fatalf("expected done sign-in probe to be skipped, got %d", len(probes))
	}
}
