package cron

import (
	"testing"
	"time"
)

func TestJitteredNextCronRunMs(t *testing.T) {
	from := time.Date(2026, 4, 30, 10, 0, 0, 0, time.Local).UnixMilli()
	cfg := DefaultCronJitterConfig

	// For a recurring task with "0 * * * *" (every hour), jitter should
	// spread within [0, RecurringCapMs) of the base time.
	next := JitteredNextCronRunMs("0 * * * *", from, "abcd1234", cfg)
	if next == 0 {
		t.Fatal("expected non-zero result")
	}
	base := NextCronRunMs("0 * * * *", from)
	if base == 0 {
		t.Fatal("base is zero")
	}
	if next < base {
		t.Errorf("jittered next (%d) should be >= base (%d)", next, base)
	}
	// Jitter should be at most RecurringCapMs (15 min) from base.
	if next-base > cfg.RecurringCapMs {
		t.Errorf("jitter too large: %d ms (cap %d)", next-base, cfg.RecurringCapMs)
	}
}

func TestJitteredNextCronRunMsNoSecondMatch(t *testing.T) {
	// Pinned date: Feb 28, 2026 at 14:30. From just after the match (Feb 28
	// 14:31), the next match is Feb 28 2027 which is ~365 days away — within
	// the 366-day search window, so jitter IS applied. Verify jitter stays
	// within expected bounds.
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli()
	cfg := DefaultCronJitterConfig

	next := JitteredNextCronRunMs("30 14 28 2 *", from, "test0001", cfg)
	base := NextCronRunMs("30 14 28 2 *", from)
	if next == 0 || base == 0 {
		t.Fatal("expected non-zero result")
	}
	if next < base {
		t.Errorf("jittered next (%d) should be >= base (%d)", next, base)
	}
	// Jitter at most RecurringCapMs.
	if next-base > cfg.RecurringCapMs {
		t.Errorf("jitter too large: %d ms (cap %d)", next-base, cfg.RecurringCapMs)
	}
}

func TestOneShotJitteredNextCronRunMs_OnHotMinute(t *testing.T) {
	// Fire time at :00 is a hot minute → jitter applies.
	from := time.Date(2026, 4, 30, 10, 0, 0, 0, time.Local).UnixMilli()
	cfg := DefaultCronJitterConfig

	// At 10:00, "0 11 * * *" fires at 11:00 → :00 is a hot minute (11:00 % 30 == 0)
	next := OneShotJitteredNextCronRunMs("0 11 * * *", from, "abcd1234", cfg)
	if next == 0 {
		t.Fatal("expected non-zero result")
	}
	base := NextCronRunMs("0 11 * * *", from)
	if next > base {
		t.Errorf("one-shot next (%d) should be <= base (%d)", next, base)
	}
	// Jitter should be at most OneShotMaxMs (90s) before base.
	if base-next > cfg.OneShotMaxMs {
		t.Errorf("jitter too large: %d ms (max %d)", base-next, cfg.OneShotMaxMs)
	}
}

func TestOneShotJitteredNextCronRunMs_OffHotMinute(t *testing.T) {
	// Fire time at :17 is not a hot minute → no jitter.
	from := time.Date(2026, 4, 30, 10, 0, 0, 0, time.Local).UnixMilli()
	cfg := DefaultCronJitterConfig

	// The minute 17 % 30 != 0, so no jitter
	next := OneShotJitteredNextCronRunMs("17 11 * * *", from, "abcd1234", cfg)
	base := NextCronRunMs("17 11 * * *", from)
	if next != base {
		t.Errorf("expected next == base (off hot minute), got %d != %d", next, base)
	}
}

func TestFindMissedTasks(t *testing.T) {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.Local).UnixMilli()

	tasks := []CronTask{
		{
			ID: "past", Cron: "0 10 * * *", CreatedAt: time.Date(2026, 4, 30, 8, 0, 0, 0, time.Local),
			Recurring: false,
		},
		{
			ID: "future", Cron: "0 14 * * *", CreatedAt: time.Date(2026, 4, 30, 8, 0, 0, 0, time.Local),
			Recurring: false,
		},
	}

	missed := FindMissedTasks(tasks, now)
	if len(missed) != 1 {
		t.Fatalf("expected 1 missed task, got %d", len(missed))
	}
	if missed[0].ID != "past" {
		t.Errorf("expected 'past' to be missed, got %q", missed[0].ID)
	}
}

func TestIsRecurringTaskAged(t *testing.T) {
	now := time.Now().UnixMilli()
	maxAge := int64(7 * 24 * 60 * 60 * 1000) // 7 days

	tests := []struct {
		name     string
		task     CronTask
		maxAgeMs int64
		want     bool
	}{
		{
			"not aged", CronTask{
				Recurring: true,
				CreatedAt: time.Now(), // just created
			}, maxAge, false,
		},
		{
			"aged", CronTask{
				Recurring: true,
				CreatedAt: time.UnixMilli(now - maxAge - 1000), // older than max
			}, maxAge, true,
		},
		{
			"unlimited age", CronTask{
				Recurring: true,
				CreatedAt: time.UnixMilli(now - maxAge - 1000),
			}, 0, false, // maxAgeMs=0 means unlimited
		},
		{
			"non-recurring not aged", CronTask{
				Recurring: false,
				CreatedAt: time.UnixMilli(now - maxAge - 1000),
			}, maxAge, false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRecurringTaskAged(tt.task, now, tt.maxAgeMs)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJitterFrac(t *testing.T) {
	// jitterFrac should return values in [0, 1).
	for i := 0; i < 100; i++ {
		f := jitterFrac("abcd1234")
		if f < 0 || f >= 1 {
			t.Errorf("jitterFrac out of range: %f", f)
		}
	}

	// Short ID should return 0.
	if f := jitterFrac("ab"); f != 0 {
		t.Errorf("expected 0 for short ID, got %f", f)
	}
}
