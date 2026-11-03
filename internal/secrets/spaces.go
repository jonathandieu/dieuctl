package secrets

import (
	"context"
	"encoding/xml"
	"net/http"
	"strings"
	"time"
)

const DefaultSpacesRegion = "nyc3"

// spacesChecker validates a Spaces access key pair with a signed ListBuckets
// (or HEAD on one bucket for keys scoped to a single bucket).
type spacesChecker struct {
	client *http.Client
	region string
	bucket string
	// endpoint overrides the https://<region>.digitaloceanspaces.com host (tests).
	endpoint string
	now      func() time.Time
}

func NewSpaces(client *http.Client, region, bucket string) Checker {
	if region == "" {
		region = DefaultSpacesRegion
	}
	return spacesChecker{client: client, region: region, bucket: bucket, now: time.Now}
}

func (s spacesChecker) Check(ctx context.Context, c Creds) Outcome {
	host := s.region + ".digitaloceanspaces.com"
	method := http.MethodGet
	if s.bucket != "" {
		host = s.bucket + "." + host
		method = http.MethodHead
	}
	base := "https://" + host + "/"
	if s.endpoint != "" {
		base = s.endpoint + "/"
	}
	req, err := http.NewRequest(method, base, nil)
	if err != nil {
		return Outcome{StatusError, "bad request"}
	}
	signV4(req, c["access_key_id"].Reveal(), c["secret_access_key"].Reveal(), s.region, "s3", s.now())

	code, body, err := send(ctx, s.client, req)
	if err != nil {
		return netOutcome(err)
	}
	switch {
	case code >= 200 && code < 300:
		return Outcome{StatusPass, "200"}
	case code == 403:
		return spacesDenied(body)
	case code == 404 && s.bucket != "":
		return Outcome{StatusFail, "404 bucket not found"}
	default:
		return classify(code)
	}
}

// spacesDenied reads only the S3 error Code and maps whitelisted codes to
// fixed strings.
func spacesDenied(body []byte) Outcome {
	var e struct {
		Code string `xml:"Code"`
	}
	_ = xml.Unmarshal(body, &e)
	switch strings.TrimSpace(e.Code) {
	case "InvalidAccessKeyId":
		return Outcome{StatusFail, "403 invalid access key id"}
	case "SignatureDoesNotMatch":
		return Outcome{StatusFail, "403 signature mismatch (wrong secret key)"}
	case "AccessDenied":
		return Outcome{StatusFail, "403 access denied (scoped key or revoked; set bucket to probe one bucket)"}
	default:
		return Outcome{StatusFail, "403 forbidden"}
	}
}
