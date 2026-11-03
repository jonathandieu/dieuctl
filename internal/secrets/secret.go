package secrets

import (
	"fmt"
	"io"
)

// Secret holds a credential value and redacts itself under every fmt verb.
// The only way to read the value is Reveal.
type Secret string

func (s Secret) Reveal() string { return string(s) }

func (Secret) String() string   { return "[REDACTED]" }
func (Secret) GoString() string { return "secrets.Secret([REDACTED])" }

func (Secret) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, "[REDACTED]") }

func (Secret) MarshalText() ([]byte, error) { return []byte("[REDACTED]"), nil }

// Creds is the set of secret values handed to a Checker, keyed by role
// ("token" for single-value credentials, "access_key_id"/"secret_access_key"
// for Spaces).
type Creds map[string]Secret
