package secrets

import "context"

type Status string

const (
	StatusPass    Status = "PASS"
	StatusFail    Status = "FAIL"
	StatusMissing Status = "MISSING"
	// StatusError means the probe was inconclusive (network, 429, 5xx).
	StatusError Status = "ERROR"
)

// Outcome is what a Checker reports. Detail must be a short fixed-vocabulary
// string; never response bodies or anything derived from the credential.
type Outcome struct {
	Status Status
	Detail string
}

// Checker validates one credential against its vendor with a cheap,
// read-only request.
type Checker interface {
	Check(ctx context.Context, creds Creds) Outcome
}

// Kind describes how a provider's credentials are pulled from an item and
// which Checker validates them.
type Kind struct {
	// Fields lists the item field labels the checker needs, keyed by role.
	// When nil, every non-empty concealed field is probed independently as
	// role "token".
	Fields map[string]string
	New    func(Params) Checker
}

// Params carries non-secret per-target settings.
type Params struct {
	Region    string
	Bucket    string
	AccountID string
}

// Result is one row of the report.
type Result struct {
	Name   string `json:"name"`
	Item   string `json:"item,omitempty"`
	Field  string `json:"field,omitempty"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
}
