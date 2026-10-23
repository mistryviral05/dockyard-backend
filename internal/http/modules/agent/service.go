package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/mistryviral05/dockyard/internal/dispatch"
	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

const (
	registrationTokenTTL = 15 * time.Minute
	heartbeatIntervalSec = 30
	offlineAfter         = 3 * heartbeatIntervalSec * time.Second
	pollSeconds          = 8

	defaultWindow   = 15 * time.Minute
	minWindow       = time.Minute
	maxWindow       = 24 * time.Hour
	targetPoints    = 300
	maxSeriesPoints = 1000

	subdomainAttempts = 20
)

type DNSProvider interface {
	UpsertAgentRecords(ctx context.Context, subdomain, ip string) (domain string, wildcard string, err error)
	AgentDomain(subdomain string) string
}

type Service struct {
	db          *sql.DB
	repo        Repository
	dns         DNSProvider
	deployments DeploymentUpdater
}

func NewService(repo Repository, db *sql.DB, dns DNSProvider, deployments DeploymentUpdater) *Service {

	return &Service{repo: repo, db: db, dns: dns, deployments: deployments}
}

func (s *Service) RecordDeployResult(ctx context.Context, agentID, deploymentID string, req *dispatch.CallbackRequest) error {
	owned, err := s.deployments.FindAgentForDeployment(ctx, deploymentID, agentID)
	if err != nil {
		return err
	}
	if !owned {
		return ErrDeploymentNotOwned
	}

	extra := map[string]interface{}{
		"container_id": nullIfEmpty(req.ContainerID),
		"url":          nullIfEmpty(req.URL),
	}

	status := statusRunningDB
	if req.Status == dispatch.StatusFailed {
		status = statusFailedDB
		extra["error_message"] = nullIfEmpty(req.Error)
	}

	if _, err := s.deployments.Transition(ctx, deploymentID, status, extra); err != nil {
		return err
	}

	return nil
}

func nullIfEmpty(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func (s *Service) Register(ctx context.Context, plainToken string, req *RegisterRequest, remoteIP string) (*RegisterResponse, error) {
	agent, err := s.repo.FindPendingByTempToken(ctx, HashToken(plainToken))
	if err != nil {
		return nil, err
	}

	ip := strings.TrimSpace(req.PublicIP)
	if ip == "" {
		ip = remoteIP
	}
	if net.ParseIP(ip) == nil {
		return nil, ErrPublicIPInvalid
	}

	subdomain, err := s.pickSubdomain(ctx, agent)
	if err != nil {
		return nil, err
	}

	newToken, tokenHash, err := GenerateToken()
	if err != nil {
		return nil, err
	}

	signingSecret, err := GenerateSigningSecret()
	if err != nil {
		return nil, err
	}

	domain, wildcard, err := s.dns.UpsertAgentRecords(ctx, subdomain, ip)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	txRepo := s.repo.WithTx(tx)

	dto := &RegisterDto{
		AgentID:       agent.Id,
		Subdomain:     subdomain,
		PublicIP:      ip,
		OSInfo:        strings.TrimSpace(req.OSInfo),
		SigningSecret: signingSecret,
	}

	if err := txRepo.CompleteRegistration(ctx, dto); err != nil {
		return nil, err
	}

	if err := txRepo.InsertToken(ctx, id.AgentTokenID(), agent.Id, tokenHash, TokenTypeJWT); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &RegisterResponse{
		AgentID:           agent.Id,
		Token:             newToken,
		SigningSecret:     signingSecret,
		Domain:            domain,
		Wildcard:          wildcard,
		Port:              agent.Port,
		HeartbeatInterval: heartbeatIntervalSec,
	}, nil
}

func (s *Service) pickSubdomain(ctx context.Context, agent *Agent) (string, error) {
	base := Slugify(agent.AgentName.String)
	if base == "" {
		base = fallbackSubdomain
	}

	candidate := base
	for attempt := 2; attempt <= subdomainAttempts; attempt++ {
		taken, err := s.repo.SubdomainTaken(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, attempt)
	}

	return "", ErrSubdomainConflict
}

func (s *Service) Create(ctx context.Context, clerkID, name, port string) (*CreateAgentResponse, error) {
	plainToken, tokenHash, err := GenerateToken()
	if err != nil {
		return nil, err
	}

	dto := &CreateAgentDto{
		ClerkId:   clerkID,
		AgentId:   id.AgentID(),
		Name:      name,
		Port:      normalizePort(port),
		TokenHash: tokenHash,
		TokenExp:  time.Now().Add(registrationTokenTTL),
	}

	created, err := s.repo.Create(ctx, dto)
	if err != nil {
		return nil, err
	}

	return &CreateAgentResponse{
		Id:             created.Id,
		Name:           created.AgentName.String,
		Status:         created.Status,
		Port:           created.Port,
		CreatedAt:      created.CreatedAt,
		Token:          plainToken, // last time this value exists server-side
		TokenExpiresAt: created.TokenExp.Time,
		TokenExpiresIn: int64(time.Until(created.TokenExp.Time).Seconds()),
		RunCommand:     BuildRunCommand(plainToken, created.Port),
	}, nil
}

func (s *Service) GetMetrics(ctx context.Context, clerkID, agentID string, q MetricsQuery) (*AgentMetricsDto, error) {
	return s.buildMetrics(ctx, clerkID, agentID, q)
}

func (s *Service) GetPublicMetrics(ctx context.Context, agentID string, q MetricsQuery) (*AgentMetricsDto, error) {
	dto, err := s.buildMetrics(ctx, "", agentID, q)
	if err != nil {
		return nil, err
	}

	dto.PublicIP = nil

	return dto, nil
}

func (s *Service) buildMetrics(ctx context.Context, clerkID, agentID string, q MetricsQuery) (*AgentMetricsDto, error) {
	m, err := s.repo.GetLatestMetrics(ctx, agentID, clerkID)
	if err != nil {
		return nil, err
	}

	window := clampWindow(q.Window)
	bucket := bucketFor(window)

	dto := &AgentMetricsDto{
		AgentID:       m.AgentID,
		Name:          m.AgentName.String,
		Status:        m.Status,
		CPUCores:      m.CPUCores,
		UptimeSec:     m.UptimeSec,
		WindowSeconds: int(window.Seconds()),
		BucketSeconds: bucket,
		PollSeconds:   pollSeconds,
		Series:        []*MetricPointDto{},
	}

	if m.PublicIP.Valid {
		dto.PublicIP = &m.PublicIP.String
	}
	if m.OSInfo.Valid {
		dto.OSInfo = &m.OSInfo.String
	}
	if m.LastHeartbeatAt.Valid {
		dto.LastHeartbeatAt = &m.LastHeartbeatAt.Time
		dto.Live = time.Since(m.LastHeartbeatAt.Time) <= offlineAfter
	}

	if m.RecordedAt.Valid {
		dto.Current = &MetricsSampleDto{
			CPUUsage:    m.CPUUsage,
			MemoryUsage: m.MemoryUsage,
			DiskUsage:   m.DiskUsage,
			Load1:       m.Load1,
			Load5:       m.Load5,
			Load15:      m.Load15,
			LoadPerCore: loadPerCore(m.Load1, m.CPUCores),
			RecordedAt:  m.RecordedAt.Time,
			AgeSeconds:  int64(time.Since(m.RecordedAt.Time).Seconds()),
		}
	}

	points, err := s.repo.GetMetricsSeries(ctx, agentID, seriesStart(window, q.Since), bucket, maxSeriesPoints)
	if err != nil {
		return nil, err
	}

	for _, p := range points {
		dto.Series = append(dto.Series, &MetricPointDto{
			T:    p.At.UTC(),
			CPU:  p.CPU,
			RAM:  p.RAM,
			Disk: p.Disk,
			Load: p.Load,
		})
	}

	return dto, nil
}

func (s *Service) AuthenticateAgent(ctx context.Context, plainToken string) (string, bool, error) {
	agentID, err := s.repo.FindAgentIDByTokenHash(ctx, HashToken(plainToken))
	if err != nil {
		if errors.Is(err, ErrAgentTokenInvalid) {
			return "", false, nil
		}
		return "", false, err
	}

	return agentID, true, nil
}

func (s *Service) RecordHeartbeat(ctx context.Context, dto *HeartbeatDto) (*HeartbeatResponse, error) {
	if dto.RecordedAt.IsZero() {
		dto.RecordedAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	txRepo := s.repo.WithTx(tx)

	if err := txRepo.UpdateHeartbeat(ctx, dto); err != nil {
		return nil, err
	}

	if err := txRepo.InsertMetrics(ctx, id.AgentMetricID(), dto); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &HeartbeatResponse{
		AgentID:           dto.AgentID,
		Status:            StatusActive,
		RecordedAt:        dto.RecordedAt,
		HeartbeatInterval: heartbeatIntervalSec,
	}, nil
}

func (s *Service) GetByID(ctx context.Context, clerkID, agentID string) (*AgentDetailDto, error) {
	a, err := s.repo.GetByID(ctx, agentID, clerkID)
	if err != nil {
		return nil, err
	}

	dto := &AgentDetailDto{
		Id:           a.Id,
		Name:         a.AgentName.String,
		Status:       a.Status,
		Port:         a.Port,
		ProjectCount: a.ProjectCount,
		CreatedAt:    a.CreatedAt,
	}
	if a.Subdomain.Valid {
		dto.Subdomain = &a.Subdomain.String
	}
	if a.LastHeartbeatAt.Valid {
		dto.LastHeartbeatAt = &a.LastHeartbeatAt.Time
	}

	return dto, nil
}

func (s *Service) Get(ctx context.Context, clerkID string) ([]*AgentDto, error) {
	agents, err := s.repo.GetByClerkId(ctx, clerkID)
	if err != nil {
		return nil, err
	}

	dtos := make([]*AgentDto, 0, len(agents))
	for _, a := range agents {
		dto := &AgentDto{
			Id:     a.Id,
			Name:   a.AgentName.String,
			Status: a.Status,
			Port:   a.Port,
		}
		if a.Subdomain.Valid {
			dto.Subdomain = &a.Subdomain.String
		}
		dtos = append(dtos, dto)
	}

	return dtos, nil
}
