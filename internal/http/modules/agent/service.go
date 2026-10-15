package agent

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mistryviral05/dockyard/internal/http/utils/id"
)

const (
	registrationTokenTTL = 15 * time.Minute
	heartbeatIntervalSec = 30
	offlineAfter         = 3 * heartbeatIntervalSec * time.Second
)

type Service struct {
	db   *sql.DB
	repo Repository
}

func NewService(repo Repository, db *sql.DB) *Service {

	return &Service{repo: repo, db: db}
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

func (s *Service) GetMetrics(ctx context.Context, clerkID, agentID string) (*AgentMetricsDto, error) {
	m, err := s.repo.GetLatestMetrics(ctx, agentID, clerkID)
	if err != nil {
		return nil, err
	}

	dto := &AgentMetricsDto{
		AgentID: m.AgentID,
		Status:  m.Status,
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
		dto.Metrics = &MetricsSampleDto{
			CPUUsage:    m.CPUUsage,
			MemoryUsage: m.MemoryUsage,
			DiskUsage:   m.DiskUsage,
			Load1:       m.Load1,
			Load5:       m.Load5,
			Load15:      m.Load15,
			LoadPerCore: loadPerCore(m.Load1, m.CPUCores),
			CPUCores:    m.CPUCores,
			UptimeSec:   m.UptimeSec,
			RecordedAt:  m.RecordedAt.Time,
			AgeSeconds:  int64(time.Since(m.RecordedAt.Time).Seconds()),
		}
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
