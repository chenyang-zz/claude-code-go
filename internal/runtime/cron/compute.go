// Package cron provides cron expression parsing and next-run-time computation.
//
// Supports the standard 5-field cron subset:
//
//	minute hour day-of-month month day-of-week
//
// Field syntax: wildcard (*), N, step (*/N), range (N-M), list (N,M,...).
// No L, W, ?, or name aliases. All times are interpreted in the process's
// local timezone — "0 9 * * *" means 9am wherever the CLI is running.
package cron

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CronFields holds the expanded set of matching values for each of the five
// cron fields.
type CronFields struct {
	Minute     []int
	Hour       []int
	DayOfMonth []int
	Month      []int
	DayOfWeek  []int
}

// fieldRange defines the inclusive min/max for a single cron field.
type fieldRange struct {
	min, max int
}

// FIELD_RANGES maps the five standard cron fields to their valid ranges.
var fieldRanges = []fieldRange{
	{0, 59},  // minute
	{0, 23},  // hour
	{1, 31},  // dayOfMonth
	{1, 12},  // month
	{0, 6},   // dayOfWeek (0=Sunday; 7 accepted as Sunday alias)
}

// ParseCronExpression parses a 5-field cron expression into expanded number
// arrays. Returns an error if any field is invalid or unsupported syntax is
// encountered.
func ParseCronExpression(expr string) (*CronFields, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}

	expanded := make([][]int, 5)
	for i := 0; i < 5; i++ {
		result, err := expandField(parts[i], fieldRanges[i])
		if err != nil {
			return nil, fmt.Errorf("field %d (%q): %w", i+1, parts[i], err)
		}
		expanded[i] = result
	}

	return &CronFields{
		Minute:     expanded[0],
		Hour:       expanded[1],
		DayOfMonth: expanded[2],
		Month:      expanded[3],
		DayOfWeek:  expanded[4],
	}, nil
}

// expandField parses a single cron field into a sorted slice of matching
// values. Supports wildcard, N, */N (step), N-M (range), N-M/N (ranged step),
// and comma-separated lists.
func expandField(field string, r fieldRange) ([]int, error) {
	out := make(map[int]struct{})

	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)

		// wildcard or */N
		if stepMatch := parseStarStep(part); stepMatch != nil {
			step := *stepMatch
			if step < 1 {
				return nil, fmt.Errorf("invalid step %d", step)
			}
			for i := r.min; i <= r.max; i += step {
				out[i] = struct{}{}
			}
			continue
		}

		// N-M or N-M/N
		if lo, hi, step, ok := parseRange(part); ok {
			isDOW := r.min == 0 && r.max == 6
			effMax := hi
			if isDOW {
				effMax = 7
			}
			if lo > hi || step < 1 || lo < r.min || hi > effMax {
				return nil, fmt.Errorf("invalid range %q", part)
			}
			for i := lo; i <= hi; i += step {
				if isDOW && i == 7 {
					out[0] = struct{}{}
				} else {
					out[i] = struct{}{}
				}
			}
			continue
		}

		// plain N
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid field value %q", part)
		}
		// dayOfWeek: accept 7 as Sunday alias → 0
		if r.min == 0 && r.max == 6 && n == 7 {
			n = 0
		}
		if n < r.min || n > r.max {
			return nil, fmt.Errorf("value %d out of range [%d, %d]", n, r.min, r.max)
		}
		out[n] = struct{}{}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no valid values in field %q", field)
	}

	result := make([]int, 0, len(out))
	for v := range out {
		result = append(result, v)
	}
	sort.Ints(result)
	return result, nil
}

// parseStarStep matches wildcard or */N. Returns nil if not a match.
func parseStarStep(part string) *int {
	if part == "*" {
		step := 1
		return &step
	}
	if strings.HasPrefix(part, "*/") {
		n, err := strconv.Atoi(part[2:])
		if err != nil {
			return nil
		}
		return &n
	}
	return nil
}

// parseRange matches N-M or N-M/N. Returns lo, hi, step, and whether it
// matched.
func parseRange(part string) (lo, hi, step int, ok bool) {
	// N-M/N
	if idx := strings.Index(part, "/"); idx >= 0 {
		rangePart := part[:idx]
		dashIdx := strings.Index(rangePart, "-")
		if dashIdx < 0 {
			return 0, 0, 0, false
		}
		l, err1 := strconv.Atoi(rangePart[:dashIdx])
		h, err2 := strconv.Atoi(rangePart[dashIdx+1:])
		s, err3 := strconv.Atoi(part[idx+1:])
		if err1 != nil || err2 != nil || err3 != nil {
			return 0, 0, 0, false
		}
		return l, h, s, true
	}
	// N-M
	dashIdx := strings.Index(part, "-")
	if dashIdx < 0 {
		return 0, 0, 0, false
	}
	l, err1 := strconv.Atoi(part[:dashIdx])
	h, err2 := strconv.Atoi(part[dashIdx+1:])
	if err1 != nil || err2 != nil {
		return 0, 0, 0, false
	}
	return l, h, 1, true
}

// ComputeNextCronRun computes the next time strictly after from that matches
// the given cron fields, using the process's local timezone. Walks forward
// minute-by-minute. Bounded at 366 days; returns nil if no match.
//
// Standard cron semantics: when both dayOfMonth and dayOfWeek are constrained
// (neither is the full range), a date matches if EITHER matches.
//
// DST: fixed-hour crons targeting a spring-forward gap skip the transition
// day. Wildcard-hour crons fire at the first valid minute after the gap.
// Fall-back repeats fire once (the step-forward logic jumps past the second
// occurrence).
func ComputeNextCronRun(fields *CronFields, from time.Time) *time.Time {
	minuteSet := toSet(fields.Minute)
	hourSet := toSet(fields.Hour)
	domSet := toSet(fields.DayOfMonth)
	monthSet := toSet(fields.Month)
	dowSet := toSet(fields.DayOfWeek)

	domWild := len(fields.DayOfMonth) == 31
	dowWild := len(fields.DayOfWeek) == 7

	// Round up to the next whole minute (strictly after from)
	t := from.Truncate(time.Minute).Add(time.Minute)

	maxIter := 366 * 24 * 60
	for i := 0; i < maxIter; i++ {
		month := int(t.Month())
		if !monthSet[month] {
			// Jump to start of next month
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
			continue
		}

		dom := t.Day()
		dow := int(t.Weekday())
		// When both dom/dow are constrained, either match is sufficient (OR)
		dayMatches := false
		switch {
		case domWild && dowWild:
			dayMatches = true
		case domWild:
			dayMatches = dowSet[dow]
		case dowWild:
			dayMatches = domSet[dom]
		default:
			dayMatches = domSet[dom] || dowSet[dow]
		}

		if !dayMatches {
			// Jump to start of next day
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
			continue
		}

		if !hourSet[t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
			continue
		}

		if !minuteSet[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}

		return &t
	}

	return nil
}

// NextCronRunMs is a convenience wrapper around ComputeNextCronRun that
// accepts a raw cron string and a time in epoch ms, returning the next run
// as epoch ms or 0 if no match.
func NextCronRunMs(cron string, fromMs int64) int64 {
	fields, err := ParseCronExpression(cron)
	if err != nil {
		return 0
	}
	from := time.UnixMilli(fromMs)
	next := ComputeNextCronRun(fields, from)
	if next == nil {
		return 0
	}
	return next.UnixMilli()
}

// toSet converts a sorted int slice to a lookup map for O(1) membership tests.
func toSet(vals []int) map[int]bool {
	m := make(map[int]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}
