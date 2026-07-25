package cluster

import "fmt"

// deriveName returns the canonical cluster name for the given coordinates.
// Format: dieubernetes-{tier}-{provider}-{region}
func deriveName(tier, provider, region string) string {
	return fmt.Sprintf("dieubernetes-%s-%s-%s", tier, provider, region)
}
