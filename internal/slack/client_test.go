package slack

import "testing"

func TestEmojiIsAlias(t *testing.T) {
	cases := []struct {
		value      string
		wantTarget string
		wantOK     bool
	}{
		{"alias:squirrel", "squirrel", true},
		{"https://my.slack.com/emoji/bowtie/46ec6f2bb0.png", "", false},
		{"alias:", "", true}, // empty target is still a well-formed alias
	}
	for _, c := range cases {
		e := Emoji{Name: "shipit", Value: c.value}
		target, ok := e.IsAlias()
		if ok != c.wantOK || target != c.wantTarget {
			t.Errorf("IsAlias(%q) = (%q, %v), want (%q, %v)", c.value, target, ok, c.wantTarget, c.wantOK)
		}
	}
}
