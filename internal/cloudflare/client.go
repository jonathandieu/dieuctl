package cloudflare

import (
	cloudflareSDK "github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/option"
)

type Client struct {
	cf     *cloudflareSDK.Client
	zoneID string
}

func New(apiToken, zoneID string) *Client {
	cf := cloudflareSDK.NewClient(option.WithAPIToken(apiToken))
	return &Client{cf: cf, zoneID: zoneID}
}
