package agentd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const (

	stateFileMode = 0o600
	stateDirMode  = 0o700
)

var ErrStateIncomplete = errors.New("state file is missing required fields")


type State struct {
	AgentID           string    `json:"agent_id"`
	ControlPlaneToken string    `json:"control_plane_token"` // bearer
	SigningSecret     string    `json:"signing_secret"`      // HMAC, symmetric
	RegisteredAt      time.Time `json:"registered_at"`
}


func LoadState(log *slog.Logger, path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Debug("state file not found, agent is unregistered",
				slog.String("path", path),
			)
			return nil, nil
		}

		return nil, fmt.Errorf("read state file %s: %w", path, err)
	}

	if err := CheckStatePerms(path); err != nil {
		log.Warn("state file permissions are too open",
			slog.String("path", path),
			slog.String("error", err.Error()),
		)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state file %s: %w", path, err)
	}

	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("state file %s: %w", path, err)
	}

	log.Info("state loaded",
		slog.String("path", path),
		slog.String("agent_id", s.AgentID),
		slog.Time("registered_at", s.RegisteredAt),
	)

	return &s, nil
}


func SaveState(log *slog.Logger, path string, s *State) error {
	if s == nil {
		return errors.New("save state: nil state")
	}
	if err := s.validate(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, stateDirMode); err != nil {
		return fmt.Errorf("create state dir %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".agent-*.json")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpName := tmp.Name()

	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()


	if err := tmp.Chmod(stateFileMode); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("chmod temp state file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write temp state file: %w", err)
	}


	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp state file: %w", err)
	}

	if runtime.GOOS == "windows" {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("replace state file %s: %w", path, err)
		}
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename state file to %s: %w", path, err)
	}

	log.Info("state saved",
		slog.String("path", path),
		slog.String("agent_id", s.AgentID),
	)

	return nil
}


func (s *State) IsRegistered() bool {
	return s != nil && s.AgentID != "" && s.ControlPlaneToken != "" && s.SigningSecret != ""
}

func (s *State) validate() error {
	if !s.IsRegistered() {
		return ErrStateIncomplete
	}
	return nil
}


func CheckStatePerms(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Errorf("state file %s is group/world readable (%#o), want %#o", path, perm, stateFileMode)
	}

	return nil
}
