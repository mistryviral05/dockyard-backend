// Package centrifugo Centrifugo ke server API se baat karta hai aur uske
// channel naam ek jagah rakhta hai.
package centrifugo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mistryviral05/dockyard/internal/config"
)

// Namespace ke naam centrifugo.json ke "namespaces" se match karte hain.
// Channel ka prefix hi batata hai kaunsa namespace lagega, isliye ye string
// badalna matlab config bhi badalna.
const (
	NamespaceDeploy  = "deploy"
	NamespaceProject = "project"
)

// DeploymentChannel ek deployment ke live logs ka channel deta hai.
func DeploymentChannel(deploymentID string) string {
	return NamespaceDeploy + ":" + deploymentID
}

// ProjectChannel ek project ke status updates ka channel deta hai.
func ProjectChannel(projectID string) string {
	return NamespaceProject + ":" + projectID
}

// SplitChannel channel ko namespace aur id me todta hai.
func SplitChannel(channel string) (namespace, id string, ok bool) {
	namespace, id, ok = strings.Cut(channel, ":")
	if !ok || namespace == "" || id == "" {
		return "", "", false
	}

	return namespace, id, true
}

type Client struct {
	apiURL string
	apiKey string
	http   *http.Client
}

func New(cfg *config.Centrifugo) *Client {
	return &Client{
		apiURL: strings.TrimSuffix(cfg.APIUrl, "/"),
		apiKey: cfg.APIKey,
		// build ke beech me centrifugo hang ho jaye to poori build na atke
		http: &http.Client{Timeout: 5 * time.Second},
	}
}

// apiError Centrifugo 200 ke saath bhi bhej deta hai, isliye body padhni padti hai.
type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type publishResponse struct {
	Error *apiError `json:"error"`
}

// Publish ek message channel pe bhejta hai. Jo log is channel pe hain unhe
// turant milta hai, aur namespace ki history me bhi chala jata hai.
func (c *Client) Publish(ctx context.Context, channel string, data interface{}) error {
	payload, err := json.Marshal(map[string]interface{}{
		"channel": channel,
		"data":    data,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.apiURL+"/api/publish", bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("centrifugo returned %s", res.Status)
	}

	var body publishResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return err
	}
	if body.Error != nil {
		return fmt.Errorf("centrifugo error %d: %s", body.Error.Code, body.Error.Message)
	}

	return nil
}
