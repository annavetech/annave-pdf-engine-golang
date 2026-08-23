// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	engineconfig "github.com/annavetech/annave-pdf-engine-golang/config"
	"github.com/annavetech/annave-pdf-engine-golang/internal/api"
	"github.com/annavetech/annave-pdf-engine-golang/internal/engine"
	"gopkg.in/yaml.v3"
)

type serverConfig struct {
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

func main() {
	var cfg serverConfig
	if err := yaml.Unmarshal(engineconfig.Server, &cfg); err != nil {
		log.Fatalf("failed to load server config: %v", err)
	}

	// Environment variables override server.yaml.
	port := os.Getenv("PORT")
	if port == "" {
		port = fmt.Sprintf("%d", cfg.Server.Port)
	}

	if os.Getenv("ANNAVE_DEBUG") == "true" {
		cfg.Server.Debug = true
	}

	corsOrigin := os.Getenv("ANNAVE_CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = cfg.CORS.AllowedOrigin
	}

	secret := os.Getenv("ANNAVE_INTERNAL_TOKEN")
	if secret == "" {
		slog.Warn("ANNAVE_INTERNAL_TOKEN is not set; internal token enforcement is disabled")
	}

	if cfg.Server.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Debug("debug logging enabled")
	}

	mux := http.NewServeMux()
	api.NewHandler(secret, corsOrigin, cfg.RateLimit.RequestsPerMinute).Register(mux)

	addr := ":" + port
	log.Printf("ANNAVE PDF Engine v%s listening on %s", engine.EngineVersion, addr)
	if err := http.ListenAndServe(addr, mux); err != nil { //nolint:gosec // adding server timeouts would be a behavior change outside the scope of this lint-compliance pass; flagged for Anna's follow-up
		log.Fatalf("server error: %v", err)
	}
}
