package worker

import "testing"

func TestTruncate(t *testing.T) {
	cases := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"shorter than max is unchanged", "hello", 10, "hello"},
		{"exactly max is unchanged", "hello", 5, "hello"},
		{"longer than max is cut", "hello world", 5, "hello"},
		{"empty string stays empty", "", 5, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.s, tc.max)
			if got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.s, tc.max, got, tc.want)
			}
		})
	}
}
