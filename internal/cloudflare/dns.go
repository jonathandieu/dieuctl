package cloudflare

import (
	"context"
	"fmt"

	cf "github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/dns"
)

// UpsertARecord creates or updates an A record. Returns the record ID.
func (c *Client) UpsertARecord(ctx context.Context, name, ip string, proxied bool) (string, error) {
	existing, err := c.findRecord(ctx, name, "A")
	if err != nil {
		return "", err
	}

	body := dns.ARecordParam{
		Name:    cf.F(name),
		Type:    cf.F(dns.ARecordTypeA),
		Content: cf.F(ip),
		Proxied: cf.F(proxied),
		TTL:     cf.F(dns.TTL1),
	}

	if existing != "" {
		_, err := c.cf.DNS.Records.Update(ctx, existing, dns.RecordUpdateParams{
			ZoneID: cf.F(c.zoneID),
			Body:   body,
		})
		if err != nil {
			return "", fmt.Errorf("update A record %q: %w", name, err)
		}
		return existing, nil
	}

	rec, err := c.cf.DNS.Records.New(ctx, dns.RecordNewParams{
		ZoneID: cf.F(c.zoneID),
		Body:   body,
	})
	if err != nil {
		return "", fmt.Errorf("create A record %q → %q: %w", name, ip, err)
	}
	return rec.ID, nil
}

// DeleteRecord removes a DNS record by ID.
func (c *Client) DeleteRecord(ctx context.Context, recordID string) error {
	_, err := c.cf.DNS.Records.Delete(ctx, recordID, dns.RecordDeleteParams{
		ZoneID: cf.F(c.zoneID),
	})
	if err != nil {
		return fmt.Errorf("delete record %q: %w", recordID, err)
	}
	return nil
}

// DeleteRecordByName finds a record by name+type and deletes it (no-op if not found).
func (c *Client) DeleteRecordByName(ctx context.Context, name, recordType string) error {
	id, err := c.findRecord(ctx, name, recordType)
	if err != nil {
		return err
	}
	if id == "" {
		return nil
	}
	return c.DeleteRecord(ctx, id)
}

func (c *Client) findRecord(ctx context.Context, name, recordType string) (string, error) {
	page, err := c.cf.DNS.Records.List(ctx, dns.RecordListParams{
		ZoneID: cf.F(c.zoneID),
		Name: cf.F(dns.RecordListParamsName{
			Exact: cf.F(name),
		}),
		Type: cf.F(dns.RecordListParamsType(recordType)),
	})
	if err != nil {
		return "", fmt.Errorf("list records for %q: %w", name, err)
	}
	if len(page.Result) == 0 {
		return "", nil
	}
	return page.Result[0].ID, nil
}
