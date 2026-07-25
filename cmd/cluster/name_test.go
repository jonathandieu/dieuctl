package cluster

import "testing"

func TestDeriveName(t *testing.T) {
	tests := []struct {
		tier, provider, region string
		want                   string
	}{
		{"platform", "do", "atl1", "dieubernetes-platform-do-atl1"},
		{"prod", "do", "nyc3", "dieubernetes-prod-do-nyc3"},
		{"stage", "do", "nyc3", "dieubernetes-stage-do-nyc3"},
		{"stage", "aws", "us-east-2", "dieubernetes-stage-aws-us-east-2"},
	}
	for _, tc := range tests {
		got := deriveName(tc.tier, tc.provider, tc.region)
		if got != tc.want {
			t.Errorf("deriveName(%q, %q, %q) = %q, want %q",
				tc.tier, tc.provider, tc.region, got, tc.want)
		}
	}
}
