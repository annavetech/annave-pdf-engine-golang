// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import "strings"

const EngineVersion = "1.1.1"
const SchemaVersion = "1"

func IsSchemaCompatible(version string) bool {
	parts := strings.SplitN(version, ".", 2)
	return parts[0] == SchemaVersion
}
