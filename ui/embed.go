// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

// Package ui embeds the single-file demo interface served at GET /.
package ui

import _ "embed"

// HTML holds the raw bytes of ui/index.html.
//
//go:embed index.html
var HTML []byte
