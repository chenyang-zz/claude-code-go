package cron

import (
	"testing"
	"time"
)

func TestParseCronExpressionValid(t *testing.T) {
	tests := []struct {
		expr   string
		fields *CronFields
	}{
		{"* * * * *", &CronFields{
			Minute: rangeSlice(0, 60), Hour: rangeSlice(0, 24),
			DayOfMonth: rangeSlice(1, 31), Month: rangeSlice(1, 12),
			DayOfWeek: rangeSlice(0, 7),
		}},
		{"*/5 * * * *", &CronFields{
			Minute: []int{0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55},
			Hour: rangeSlice(0, 24), DayOfMonth: rangeSlice(1, 31),
			Month: rangeSlice(1, 12), DayOfWeek: rangeSlice(0, 7),
		}},
		{"0 9 * * *", &CronFields{
			Minute: []int{0}, Hour: []int{9},
			DayOfMonth: rangeSlice(1, 31), Month: rangeSlice(1, 12),
			DayOfWeek: rangeSlice(0, 7),
		}},
		{"30 14 28 2 *", &CronFields{
			Minute: []int{30}, Hour: []int{14},
			DayOfMonth: []int{28}, Month: []int{2},
			DayOfWeek: rangeSlice(0, 7),
		}},
		{"0 9 * * 1-5", &CronFields{
			Minute: []int{0}, Hour: []int{9},
			DayOfMonth: rangeSlice(1, 31), Month: rangeSlice(1, 12),
			DayOfWeek: []int{1, 2, 3, 4, 5},
		}},
		{"0,30 9,17 * * *", &CronFields{
			Minute: []int{0, 30}, Hour: []int{9, 17},
			DayOfMonth: rangeSlice(1, 31), Month: rangeSlice(1, 12),
			DayOfWeek: rangeSlice(0, 7),
		}},
	}

	for _, tt := range tests {
		fields, err := ParseCronExpression(tt.expr)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tt.expr, err)
			continue
		}
		if !intSliceEqual(fields.Minute, tt.fields.Minute) {
			t.Errorf("%s: minute: got %v, want %v", tt.expr, fields.Minute, tt.fields.Minute)
		}
		if !intSliceEqual(fields.Hour, tt.fields.Hour) {
			t.Errorf("%s: hour: got %v, want %v", tt.expr, fields.Hour, tt.fields.Hour)
		}
	}
}

func TestParseCronExpressionInvalid(t *testing.T) {
	tests := []string{
		"",            // empty
		"* * * *",     // 4 fields
		"* * * * * *", // 6 fields
		"60 * * * *",  // invalid minute
		"* 24 * * *",  // invalid hour
		"* * 32 * *",  // invalid day
		"* * * 13 *",  // invalid month
		"* * * * 8",   // invalid DOW
		"invalid",     // garbage
	}

	for _, expr := range tests {
		_, err := ParseCronExpression(expr)
		if err == nil {
			t.Errorf("%q: expected error, got nil", expr)
		}
	}
}

func TestComputeNextCronRun(t *testing.T) {
	// Reference time: 2026-04-30 10:00:00
	ref := time.Date(2026, 4, 30, 10, 0, 0, 0, time.Local)

	tests := []struct {
		name string
		cron string
		want time.Time
	}{
		{"every minute", "* * * * *", time.Date(2026, 4, 30, 10, 1, 0, 0, time.Local)},
		{"daily 9am", "0 9 * * *", time.Date(2026, 5, 1, 9, 0, 0, 0, time.Local)},
		{"next hour at :30", "30 * * * *", time.Date(2026, 4, 30, 10, 30, 0, 0, time.Local)},
		{"every 5 min", "*/5 * * * *", time.Date(2026, 4, 30, 10, 5, 0, 0, time.Local)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, err := ParseCronExpression(tt.cron)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			next := ComputeNextCronRun(fields, ref)
			if next == nil {
				t.Fatal("expected non-nil result")
			}
			if !next.Equal(tt.want) {
				t.Errorf("got %v, want %v", next, tt.want)
			}
		})
	}
}

func TestComputeNextCronRunPinnedDate(t *testing.T) {
	// "30 14 28 2 *" = Feb 28 at 14:30
	// From Jan 1, it should return Feb 28 of the same year if in the future.
	ref := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	fields, _ := ParseCronExpression("30 14 28 2 *")
	next := ComputeNextCronRun(fields, ref)
	if next == nil {
		t.Fatal("expected non-nil result")
	}
	expected := time.Date(2026, 2, 28, 14, 30, 0, 0, time.Local)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestNextCronRunMs(t *testing.T) {
	from := time.Date(2026, 4, 30, 10, 0, 0, 0, time.Local).UnixMilli()
	next := NextCronRunMs("*/5 * * * *", from)
	if next <= from {
		t.Errorf("expected next > from, got %d <= %d", next, from)
	}
}

func rangeSlice(start, endExcl int) []int {
	s := make([]int, 0, endExcl-start)
	for i := start; i < endExcl; i++ {
		s = append(s, i)
	}
	return s
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
