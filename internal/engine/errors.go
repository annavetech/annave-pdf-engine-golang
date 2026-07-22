// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package engine

import "fmt"

type EngineStage string

const (
	StageInput      EngineStage = "input"
	StageParser     EngineStage = "parser"
	StageValidation EngineStage = "validation"
	StageLayout     EngineStage = "layout"
	StagePagination EngineStage = "pagination"
	StageRender     EngineStage = "render"
)

type AnnaveError struct {
	Code    string      `json:"code"`
	Stage   EngineStage `json:"stage"`
	Message string      `json:"message"`
}

func (e *AnnaveError) Error() string {
	return fmt.Sprintf("[%s/%s] %s", e.Stage, e.Code, e.Message)
}

func NewError(code string, stage EngineStage, message string) *AnnaveError {
	return &AnnaveError{Code: code, Stage: stage, Message: message}
}
