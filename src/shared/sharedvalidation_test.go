package shared

import "testing"

func TestValidPresenceStatus(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{PresenceActive, true},
		{PresenceAway, true},
		{"", false},
		{"hacking", false},
		{"Active", false}, // case-sensitive on the wire
	}
	for _, c := range cases {
		if got := ValidPresenceStatus(c.status); got != c.want {
			t.Errorf("ValidPresenceStatus(%q) = %v, want %v", c.status, got, c.want)
		}
	}
}
