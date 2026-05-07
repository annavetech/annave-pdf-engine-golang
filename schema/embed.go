// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

// Package schema embeds the JSON Schema definitions shipped with the engine.
package schema

import _ "embed"

// ErrorV1 holds the raw bytes of schema/error.v1.schema.json.
//
//go:embed error.v1.schema.json
var ErrorV1 []byte

// DocumentV1 holds the raw bytes of schema/document.v1.schema.json.
//
//go:embed document.v1.schema.json
var DocumentV1 []byte
