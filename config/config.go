// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

// Package config embeds the YAML configuration files that ship with the binary.
// All values are loaded once at startup by the engine; no external files are
// required at runtime.
package config

import _ "embed"

// Style holds the raw bytes of config/style.yaml.
//
//go:embed style.yaml
var Style []byte

// Limits holds the raw bytes of config/limits.yaml.
//
//go:embed limits.yaml
var Limits []byte

// Messages holds the raw bytes of config/messages.yaml.
//
//go:embed messages.yaml
var Messages []byte

// Server holds the raw bytes of config/server.yaml.
//
//go:embed server.yaml
var Server []byte
