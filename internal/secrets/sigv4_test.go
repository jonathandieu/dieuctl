package secrets

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// AWS documentation example: "GET Object" with a Range header.
func TestSignV4AWSVector(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	req.Header.Set("Range", "bytes=0-9")
	now := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)

	signV4(req, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", "s3", now)

	got := req.Header.Get("Authorization")
	want := "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request," +
		" SignedHeaders=host;range;x-amz-content-sha256;x-amz-date," +
		" Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got != want {
		t.Fatalf("Authorization\n got %s\nwant %s", got, want)
	}
}

func TestSecretRedaction(t *testing.T) {
	const raw = "sk-super-secret"
	s := Secret(raw)
	c := Creds{"token": s}
	var out []string
	for _, f := range []string{"%v", "%s", "%+v", "%#v", "%q"} {
		out = append(out, fmt.Sprintf(f, s), fmt.Sprintf(f, c), fmt.Sprintf(f, []Secret{s}))
	}
	out = append(out, s.String())
	if joined := strings.Join(out, "|"); strings.Contains(joined, raw) {
		t.Fatalf("secret leaked: %s", joined)
	}
	if s.Reveal() != raw {
		t.Fatal("Reveal must return the value")
	}
}
