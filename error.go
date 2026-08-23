// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package pdfengine

import (
	"errors"
	"fmt"

	"github.com/annavetech/annave-pdf-engine-golang/internal/engine"
)

// Stage identifies which part of the conversion pipeline produced an Error.
type Stage string

const (
	StageInput      Stage = "input"
	StageParser     Stage = "parser"
	StageValidation Stage = "validation"
	StageLayout     Stage = "layout"
	StagePagination Stage = "pagination"
	StageRender     Stage = "render"
)

// Error describes a failure returned by Convert. Code is a stable,
// machine-readable identifier (for example "ENGINE_ERR_FILE_TOO_LARGE").
// Message is human-readable and may change between versions — branch on
// Code, not on Message. Use errors.As to obtain an *Error from an error
// returned by Convert.
type Error struct {
	Code    string
	Stage   Stage
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("[%s/%s] %s", e.Stage, e.Code, e.Message)
}

// translateError copies the fields of an internal *engine.AnnaveError into
// a public *Error. It never returns the internal error itself, so a caller
// doing errors.As on an internal type can never succeed against a value
// this package returns.
func translateError(err error) error {
	var ae *engine.AnnaveError
	if errors.As(err, &ae) {
		return &Error{Code: ae.Code, Stage: toStage(ae.Stage), Message: ae.Message}
	}
	return err
}

func toStage(s engine.EngineStage) Stage {
	switch s {
	case engine.StageInput:
		return StageInput
	case engine.StageParser:
		return StageParser
	case engine.StageValidation:
		return StageValidation
	case engine.StageLayout:
		return StageLayout
	case engine.StagePagination:
		return StagePagination
	case engine.StageRender:
		return StageRender
	default:
		return Stage(s)
	}
}
