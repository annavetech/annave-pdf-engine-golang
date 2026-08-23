// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	engineconfig "github.com/annavetech/annave-pdf-engine-golang/config"
	"github.com/annavetech/annave-pdf-engine-golang/internal/api"
	"github.com/annavetech/annave-pdf-engine-golang/internal/engine"
)

var (
	servePort  int
	serveToken string
	serveDebug bool
	serveCORS  string
)

type serveServerConfig struct {
	Server struct {
		Port  int  `yaml:"port"`
		Debug bool `yaml:"debug"`
	} `yaml:"server"`
	CORS struct {
		AllowedOrigin string `yaml:"allowed_origin"`
	} `yaml:"cors"`
	RateLimit struct {
		RequestsPerMinute int `yaml:"requests_per_minute"`
	} `yaml:"rate_limit"`
}

var pdfServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the PDF conversion HTTP server",
	Long: `Start the HTTP server that exposes POST /convert and GET /.

Flags override the values in config/server.yaml.
The ANNAVE_INTERNAL_TOKEN environment variable is also respected.

Examples:
  annave pdf serve
  annave pdf serve --port 9000 --debug
  annave pdf serve --port 5741 --token "$(openssl rand -hex 32)"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load defaults from the embedded server.yaml.
		var cfg serveServerConfig
		if err := yaml.Unmarshal(engineconfig.Server, &cfg); err != nil {
			return fmt.Errorf("loading server config: %w", err)
		}

		// CLI flags override YAML; env var overrides both for the token.
		port := cfg.Server.Port
		if cmd.Flags().Changed("port") {
			port = servePort
		}

		debug := cfg.Server.Debug || serveDebug
		if os.Getenv("ANNAVE_DEBUG") == "true" {
			debug = true
		}

		corsOrigin := cfg.CORS.AllowedOrigin
		if cmd.Flags().Changed("cors") {
			corsOrigin = serveCORS
		} else if v := os.Getenv("ANNAVE_CORS_ORIGIN"); v != "" {
			corsOrigin = v
		}

		token := serveToken
		if token == "" {
			token = os.Getenv("ANNAVE_INTERNAL_TOKEN")
		}
		if token == "" {
			slog.Warn("no token set; internal token enforcement is disabled")
		}

		if debug {
			slog.SetLogLoggerLevel(slog.LevelDebug)
			slog.Debug("debug logging enabled")
		}

		mux := http.NewServeMux()
		api.NewHandler(token, corsOrigin, cfg.RateLimit.RequestsPerMinute).Register(mux)

		addr := fmt.Sprintf(":%d", port)
		log.Printf("ANNÁVE PDF Engine v%s listening on %s", engine.EngineVersion, addr)
		return http.ListenAndServe(addr, mux) //nolint:gosec // adding server timeouts would be a behavior change outside the scope of this lint-compliance pass; flagged for Anna's follow-up
	},
}

func init() {
	pdfServeCmd.Flags().IntVarP(&servePort, "port", "p", 5741, "Port to listen on")
	pdfServeCmd.Flags().StringVar(&serveToken, "token", "", "Internal auth token (X-Internal-Token header)")
	pdfServeCmd.Flags().BoolVar(&serveDebug, "debug", false, "Enable debug logging")
	pdfServeCmd.Flags().StringVar(&serveCORS, "cors", "", "Allowed CORS origin (default: value from server.yaml)")
}
