package realtime

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/mistryviral05/dockyard/internal/config"
	"github.com/mistryviral05/dockyard/internal/http/modules/deployment"
	"github.com/mistryviral05/dockyard/internal/http/utils/centrifugo"
	"github.com/mistryviral05/dockyard/internal/http/utils/signer"
)

type Service struct {
	cfg    *config.Centrifugo
	signer *signer.Signer
	repo   deployment.Repository
	logger *slog.Logger
}

func NewService(cfg *config.Centrifugo, sign *signer.Signer, repo deployment.Repository, logger *slog.Logger) *Service {
	return &Service{cfg: cfg, signer: sign, repo: repo, logger: logger}
}

// ConnectionToken wo short lived JWT deta hai jisse browser Centrifugo se
// connect karta hai. Ismein koi channel nahi hai - sirf "ye kaun hai".
func (s *Service) ConnectionToken(clerkID string) (*ConnectionResponse, error) {
	expiresAt := time.Now().Add(s.cfg.TokenTTL)

	token, err := s.signer.Sign(connectionClaims{Sub: clerkID}, expiresAt)
	if err != nil {
		return nil, err
	}

	return &ConnectionResponse{
		Token:     token,
		Url:       s.cfg.WSUrl,
		ExpiresIn: int64(time.Until(expiresAt).Seconds()),
	}, nil
}

// CanSubscribe har subscribe attempt pe Centrifugo poochta hai. Yahi wo jagah
// hai jahan tay hota hai ki ek user kisi doosre ke deployment ke logs na
// padh sake, isliye check hamesha ownership join se hota hai.
func (s *Service) CanSubscribe(ctx context.Context, clerkID, channel string) bool {
	if clerkID == "" {
		return false
	}

	namespace, id, ok := centrifugo.SplitChannel(channel)
	if !ok {
		return false
	}

	switch namespace {
	case centrifugo.NamespaceDeploy:
		// GetByID ka query users tak join karta hai, to row wapas aana hi
		// iska matlab hai ki deployment isi user ka hai
		_, err := s.repo.GetByID(ctx, id, clerkID)
		if err == nil {
			return true
		}
		if !errors.Is(err, deployment.ErrDeploymentNotFound) {
			s.logger.Error("could not check the channel ownership",
				slog.String("clerk_id", clerkID),
				slog.String("channel", channel),
				slog.String("error", err.Error()),
			)
		}
		return false

	default:
		// project namespace centrifugo.json me hai par uske liye abhi
		// ownership query nahi hai, isliye deny
		s.logger.Warn("subscribe denied for an unsupported namespace",
			slog.String("clerk_id", clerkID),
			slog.String("channel", channel),
		)
		return false
	}
}
