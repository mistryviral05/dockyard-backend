package agent

import (
	_ "embed"
	"strings"
)

//go:embed install.sh
var installScript string

type InstallConfig struct {
	ServerURL string
	Image     string
	ACMEEmail string
}

func RenderInstallScript(cfg InstallConfig) string {
	return strings.NewReplacer(
		"__SERVER_URL__", strings.TrimRight(cfg.ServerURL, "/"),
		"__AGENT_IMAGE__", cfg.Image,
		"__ACME_EMAIL__", cfg.ACMEEmail,
	).Replace(installScript)
}
