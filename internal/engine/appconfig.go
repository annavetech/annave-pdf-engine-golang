// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"fmt"
	"log"
	"strings"

	engineconfig "annave.tech/pdf-engine/config"
	"gopkg.in/yaml.v3"
)

// appLimits holds the active input/output limits loaded from config/limits.yaml.
var appLimits limitsConfig

// appMessages holds all user-facing error messages loaded from config/messages.yaml.
var appMessages messagesConfig

func init() {
	if err := yaml.Unmarshal(engineconfig.Limits, &appLimits); err != nil {
		log.Fatalf("engine: failed to load limits config: %v", err)
	}
	if err := yaml.Unmarshal(engineconfig.Messages, &appMessages); err != nil {
		log.Fatalf("engine: failed to load messages config: %v", err)
	}
}

type limitsConfig struct {
	Input struct {
		MaxFileSizeBytes int `yaml:"max_file_size_bytes"`
		MaxInputChars    int `yaml:"max_input_chars"`
	} `yaml:"input"`
	Document struct {
		MaxNodes int `yaml:"max_nodes"`
		MaxPages int `yaml:"max_pages"`
	} `yaml:"document"`
}

type messagesConfig struct {
	Errors map[string]string `yaml:"errors"`
}

// msg returns the configured message for the given error code, interpolating
// any {key} placeholders with the provided values.
func msg(code string, args ...any) string {
	template, ok := appMessages.Errors[code]
	if !ok {
		return fmt.Sprintf("unknown error code: %s", code)
	}
	// Simple placeholder interpolation: args are expected in pairs (key, value).
	for i := 0; i+1 < len(args); i += 2 {
		k := fmt.Sprintf("{%v}", args[i])
		v := fmt.Sprintf("%v", args[i+1])
		template = strings.ReplaceAll(template, k, v)
	}
	return template
}
