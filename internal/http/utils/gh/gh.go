package gh

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/bradleyfalzon/ghinstallation/v2"
	gogithub "github.com/google/go-github/v66/github"
	"github.com/mistryviral05/dockyard/internal/config"
)

// Client GitHub App ki chaabi rakhta hai. Ek baar banao aur sabko share karo:
// private key sirf yahan padhi jaati hai, aur har installation ka token
// transport ke andar cache rehta hai.
type Client struct {
	appID  int64
	appsTr *ghinstallation.AppsTransport

	mu         sync.RWMutex
	transports map[string]*ghinstallation.Transport
}

func NewClient(cfg *config.Github) (*Client, error) {
	appID, err := strconv.ParseInt(cfg.AppId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("github app_id is not a number: %w", err)
	}

	// key yahin parse hoti hai, taki galat path ya kharab pem
	// startup pe pakda jaye, kisi deploy ke beech me nahi
	appsTr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appID, cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load the github app private key: %w", err)
	}

	return &Client{
		appID:      appID,
		appsTr:     appsTr,
		transports: make(map[string]*ghinstallation.Transport),
	}, nil
}

// transport ek installation ka transport deta hai, cache se ya naya banakar.
// Token isi ke andar rehta hai aur expiry se pehle apne aap refresh hota hai,
// isliye har call pe naya transport banana token cache ko bekaar kar dega.
func (c *Client) transport(installationID string) (*ghinstallation.Transport, error) {
	c.mu.RLock()
	tr, ok := c.transports[installationID]
	c.mu.RUnlock()
	if ok {
		return tr, nil
	}

	id, err := strconv.ParseInt(installationID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("installation id is not a number: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// do goroutine ek saath yahan aa sakti hain, dobara dekh lo
	if tr, ok := c.transports[installationID]; ok {
		return tr, nil
	}

	tr = ghinstallation.NewFromAppsTransport(c.appsTr, id)
	c.transports[installationID] = tr

	return tr, nil
}

// InstallationToken wo token deta hai jo git clone ke URL me jaata hai.
// Ye ~1 ghante me expire hota hai, isliye ise kahin store mat karo -
// har build me naya maango, cache transport khud sambhalta hai.
func (c *Client) InstallationToken(ctx context.Context, installationID string) (string, error) {
	tr, err := c.transport(installationID)
	if err != nil {
		return "", err
	}

	token, err := tr.Token(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to mint the installation token: %w", err)
	}

	return token, nil
}

// APIClient ek go-github client deta hai jo is installation ki taraf se
// baat karta hai. Auth apne aap lagta hai.
func (c *Client) APIClient(installationID string) (*gogithub.Client, error) {
	tr, err := c.transport(installationID)
	if err != nil {
		return nil, err
	}

	return gogithub.NewClient(&http.Client{Transport: tr}), nil
}

type Commit struct {
	Sha     string
	Message string
}

// LatestCommit branch ka HEAD deta hai. Manual deploy me commit_sha khali
// hota hai, aur image ka tag usi se banta hai.
func (c *Client) LatestCommit(ctx context.Context, installationID, repoFullName, branch string) (*Commit, error) {
	owner, repo, err := SplitFullName(repoFullName)
	if err != nil {
		return nil, err
	}

	api, err := c.APIClient(installationID)
	if err != nil {
		return nil, err
	}

	b, _, err := api.Repositories.GetBranch(ctx, owner, repo, branch, 1)
	if err != nil {
		return nil, fmt.Errorf("failed to read the branch: %w", err)
	}

	return &Commit{
		Sha:     b.GetCommit().GetSHA(),
		Message: b.GetCommit().GetCommit().GetMessage(),
	}, nil
}

// CloneURL private repo ke liye authenticated URL banata hai.
// "x-access-token" fixed hai, GitHub isi username ko pehchanta hai.
func CloneURL(token, repoFullName string) string {
	return fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, repoFullName)
}

func SplitFullName(repoFullName string) (string, string, error) {
	owner, repo, ok := strings.Cut(repoFullName, "/")
	if !ok || owner == "" || repo == "" {
		return "", "", fmt.Errorf("bad repository name: %q", repoFullName)
	}

	return owner, repo, nil
}

// Redact token ko text se hata deta hai. git apne error message me poora
// URL chhap deta hai, aur wo URL deployment_logs me jaata hai - jahan se
// user ek aisa token padh sakta hai jo poori installation kholta hai.
func Redact(text, token string) string {
	if token == "" {
		return text
	}

	return strings.ReplaceAll(text, token, "***")
}
