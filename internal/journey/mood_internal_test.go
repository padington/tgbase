package journey

import (
	"testing"
	"time"

	"github.com/padington/tgbase/internal/screening"
)

// moodAgo branch coverage: same-day, days, weeks. The flow tests can only
// exercise "today" (they cannot travel in time), so the rendering rules are
// pinned here.
func TestMoodAgoRendering(t *testing.T) {
	c := &screening.MoodContent{}
	c.Module.Results.AgoToday = "today"
	c.Module.Results.AgoDays = "{n} d ago"
	c.Module.Results.AgoWeeks = "{n} w ago"

	base := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		delta time.Duration
		want  string
	}{
		{0, "today"},
		{6 * time.Hour, "today"},
		{24 * time.Hour, "1 d ago"},
		{6 * 24 * time.Hour, "6 d ago"},
		{7 * 24 * time.Hour, "1 w ago"},
		{20 * 24 * time.Hour, "2 w ago"},
		{30 * 24 * time.Hour, "4 w ago"},
	}
	for _, tc := range cases {
		if got := moodAgo(c, base, base.Add(tc.delta)); got != tc.want {
			t.Errorf("moodAgo(+%v) = %q, want %q", tc.delta, got, tc.want)
		}
	}
}
