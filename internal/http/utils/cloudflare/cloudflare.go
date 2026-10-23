package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mistryviral05/dockyard/internal/config"
)

const apiBase = "https://api.cloudflare.com/client/v4"

var ErrAPIFailed = errors.New("cloudflare api call failed")

type Client struct {
	token      string
	zoneID     string
	baseDomain string
	ttl        int
	http       *http.Client
	logger     *slog.Logger
}

func New(cfg *config.Cloudflare, logger *slog.Logger) *Client {
	return &Client{
		token:      cfg.APIToken,
		zoneID:     cfg.ZoneID,
		baseDomain: strings.Trim(cfg.BaseDomain, "."),
		ttl:        cfg.RecordTTL,
		http:       &http.Client{Timeout: cfg.Timeout},
		logger:     logger,
	}
}

func (c *Client) BaseDomain() string {
	return c.baseDomain
}

func (c *Client) AgentDomain(subdomain string) string {
	return subdomain + "." + c.baseDomain
}

func (c *Client) WildcardDomain(subdomain string) string {
	return "*." + c.AgentDomain(subdomain)
}

type dnsRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

type listResponse struct {
	Success bool        `json:"success"`
	Errors  []apiError  `json:"errors"`
	Result  []dnsRecord `json:"result"`
}

type writeResponse struct {
	Success bool       `json:"success"`
	Errors  []apiError `json:"errors"`
	Result  dnsRecord  `json:"result"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *Client) UpsertAgentRecords(ctx context.Context, subdomain, ip string) (domain, wildcard string, err error) {
	domain = c.AgentDomain(subdomain)
	wildcard = c.WildcardDomain(subdomain)

	for _, name := range []string{domain, wildcard} {
		if err := c.upsertA(ctx, name, ip); err != nil {
			return "", "", err
		}
	}

	c.logger.Info("dns records ready",
		slog.String("domain", domain),
		slog.String("wildcard", wildcard),
		slog.String("ip", ip),
	)

	return domain, wildcard, nil
}

func (c *Client) upsertA(ctx context.Context, name, ip string) error {
	existing, err := c.findA(ctx, name)
	if err != nil {
		return err
	}

	record := dnsRecord{
		Type:    "A",
		Name:    name,
		Content: ip,
		TTL:     c.ttl,
		Proxied: false,
	}

	if existing == "" {
		return c.send(ctx, http.MethodPost,
			fmt.Sprintf("%s/zones/%s/dns_records", apiBase, c.zoneID), record)
	}

	return c.send(ctx, http.MethodPut,
		fmt.Sprintf("%s/zones/%s/dns_records/%s", apiBase, c.zoneID, existing), record)
}

func (c *Client) findA(ctx context.Context, name string) (string, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records?type=A&name=%s", apiBase, c.zoneID, name)

	body, err := c.do(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	var res listResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return "", fmt.Errorf("decode dns list: %w", err)
	}
	if !res.Success {
		return "", wrapErrors("list dns records", res.Errors)
	}
	if len(res.Result) == 0 {
		return "", nil
	}

	return res.Result[0].ID, nil
}

func (c *Client) send(ctx context.Context, method, url string, record dnsRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode dns record: %w", err)
	}

	body, err := c.do(ctx, method, url, payload)
	if err != nil {
		return err
	}

	var res writeResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return fmt.Errorf("decode dns write: %w", err)
	}
	if !res.Success {
		return wrapErrors("write dns record "+record.Name, res.Errors)
	}

	return nil
}

func (c *Client) do(ctx context.Context, method, url string, payload []byte) ([]byte, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("build cloudflare request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call cloudflare: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("read cloudflare response: %w", err)
	}

	if res.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: status %d", ErrAPIFailed, res.StatusCode)
	}

	return body, nil
}

func wrapErrors(action string, errs []apiError) error {
	if len(errs) == 0 {
		return fmt.Errorf("%w: %s", ErrAPIFailed, action)
	}

	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, fmt.Sprintf("%d %s", e.Code, e.Message))
	}

	return fmt.Errorf("%w: %s: %s", ErrAPIFailed, action, strings.Join(parts, ", "))
}
