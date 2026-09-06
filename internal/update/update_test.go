package update

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{{"1.0.4", "1.0.3", true}, {"1.0.10", "1.0.9", true}, {"1.0.3", "1.0.3", false}, {"1.0.2", "1.0.3", false}, {"1.0.4", "dev", false}, {"v2.0.0", "1.9.9", true}}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%s,%s)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}
