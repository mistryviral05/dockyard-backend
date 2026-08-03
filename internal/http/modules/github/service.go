package github

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	gogithub "github.com/google/go-github/v66/github"
	"github.com/mistryviral05/dockyard/internal/config"
	"github.com/mistryviral05/dockyard/internal/http/utils/signer"
)

type Service struct {
	db     *sql.DB
	cfg    *config.Config
	signer *signer.Signer
	logger *slog.Logger
	repo   Repository
}

var ErrStateMismatch = errors.New("state does not match authenticated user")

func NewService(db *sql.DB, cfg *config.Config, repo Repository, signer *signer.Signer, logger *slog.Logger) *Service {
	return &Service{
		db:     db,
		cfg:    cfg,
		signer: signer,
		logger: logger,
		repo:   repo,
	}
}

type StateClaims struct {
	ClerkId string `json:"clerk_id"`
}

func (s *Service) GetRepositories(ctx context.Context, instRowId, clerkId string) ([]*gogithub.Repository, error) {
	installationIdStr, err := s.repo.GetInstallationId(ctx, instRowId, clerkId)
	if err != nil {
		return nil, err
	}

	installationId, err := strconv.ParseInt(installationIdStr, 10, 64)
	if err != nil {
		return nil, err
	}

	appId, err := strconv.ParseInt(s.cfg.Github.AppId, 10, 64)
	if err != nil {
		return nil, err
	}

	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appId, installationId, s.cfg.Github.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	client := gogithub.NewClient(&http.Client{Transport: itr})

	repos := []*gogithub.Repository{}
	opt := &gogithub.ListOptions{PerPage: 100}
	for {
		list, resp, err := client.Apps.ListRepos(ctx, opt)
		if err != nil {
			return nil, err
		}
		repos = append(repos, list.Repositories...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return repos, nil
}

func (s *Service) GetInstallations(ctx context.Context, clerkId string) ([]InstallationDto, error) {
	return s.repo.GetInstallations(ctx, clerkId)
}

func (s *Service) CreateInstallation(ctx context.Context, p ConnectRequest) error {
	// state verify
	var stateClaims StateClaims
	if err := s.signer.Verify(p.State, &stateClaims); err != nil {
		s.logger.Error("Invalid State token")
		return fmt.Errorf("invalid state token")
	}
	if stateClaims.ClerkId != p.ClerkId {
		s.logger.Error("State token does not match authenticated user")
		return ErrStateMismatch
	}

	installationId, err := strconv.ParseInt(p.InstallationId, 10, 64)
	if err != nil {
		s.logger.Error("Invalid Installation ID")
		return fmt.Errorf("invalid installation id")
	}

	appId, err := strconv.ParseInt(s.cfg.Github.AppId, 10, 64)
	if err != nil {
		s.logger.Error("Invalid App ID")
		return fmt.Errorf("invalid app id")
	}

	appTr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appId, s.cfg.Github.PrivateKeyPath)

	if err != nil {
		return err
	}
	inst, _, err := gogithub.NewClient(&http.Client{Transport: appTr}).Apps.GetInstallation(ctx, installationId)

	if err != nil {
		return err
	}

	// parsing permsions in the jsonb
	permsJSON, err := json.Marshal(inst.GetPermissions())
	if err != nil {
		s.logger.Error("faild to parse permisions into the json")
		return err
	}

	// guard if the json is empty
	if len(permsJSON) == 0 || string(permsJSON) == "null" {
		permsJSON = []byte("{}")
	}

	_, err = s.repo.CreateInstallation(ctx, ConnectDto{
		ClerkId:             p.ClerkId,
		InstallationId:      p.InstallationId,
		AccountLogin:        inst.GetAccount().GetLogin(),
		AccountType:         inst.GetAccount().GetType(),
		RepositorySelection: inst.GetRepositorySelection(),
		Permissions:         string(permsJSON),
	})

	return err
}

func (s *Service) CreateInstallationLink(clerkId string) (string, error) {
	claims := StateClaims{
		ClerkId: clerkId,
	}
	token, err := s.signer.Sign(claims, time.Now().Add(60*time.Minute))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://github.com/apps/%s/installations/new?state=%s", s.cfg.Github.AppSlug, token), nil
}
