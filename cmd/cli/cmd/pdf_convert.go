// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"annave.tech/pdf-engine/internal/engine"
	"annave.tech/pdf-engine/internal/parser"
)

var (
	convertOutput string
	convertFormat string
	convertStdin  bool
	convertStyle  string
)

var pdfConvertCmd = &cobra.Command{
	Use:   "convert [file]",
	Short: "Convert a document to PDF",
	Long: `Convert a document to PDF without starting an HTTP server.

Reads from a file argument, or from stdin with --stdin.
Writes to the path given by --output, or to stdout if --output is omitted.

Examples:
  annave pdf convert README.md -o output.pdf
  annave pdf convert report.docx -o report.pdf
  cat data.csv | annave pdf convert --stdin --format csv > table.pdf
  annave pdf convert --stdin --format md < spec.md > spec.pdf`,
	Args: func(cmd *cobra.Command, args []string) error {
		if convertStdin && len(args) > 0 {
			return fmt.Errorf("cannot use --stdin together with a file argument")
		}
		if !convertStdin && len(args) != 1 {
			return fmt.Errorf("provide a file argument, or use --stdin to read from stdin")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		var input []byte
		var format parser.InputFormat
		var sourceName string

		if convertStdin {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			input = data
			sourceName = "stdin"
		} else {
			path := args[0]
			data, err := os.ReadFile(path) //nolint:gosec // path is the CLI's positional input-file argument, provided directly by the operator
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
			input = data
			sourceName = filepath.Base(path)
			// Auto-detect format from extension when --format is not set.
			if convertFormat == "" {
				format = parser.FormatFromExtension(path)
			}
		}

		if convertFormat != "" {
			format = parser.InputFormat(strings.ToLower(convertFormat))
		}
		if format == "" {
			format = parser.FormatAuto
		}

		var runOpts []engine.RunOption
		if convertStyle != "" {
			var override engine.StyleOverride
			if err := json.Unmarshal([]byte(convertStyle), &override); err != nil {
				return fmt.Errorf("invalid --style JSON: %w", err)
			}
			runOpts = append(runOpts, engine.WithStyleOverride(&override))
		}

		pipe := engine.NewPipeline()
		pdfBytes, err := pipe.Run(string(input), format, runOpts...)
		if err != nil {
			if ae, ok := err.(*engine.AnnaveError); ok {
				return fmt.Errorf("%s: %s", ae.Code, ae.Message)
			}
			return err
		}

		if convertOutput == "" {
			// Write PDF to stdout. Useful for piping: annave pdf convert ... > out.pdf
			_, err = os.Stdout.Write(pdfBytes)
			return err
		}

		if err := os.WriteFile(convertOutput, pdfBytes, 0600); err != nil {
			return fmt.Errorf("writing %s: %w", convertOutput, err)
		}
		fmt.Fprintf(os.Stderr, "Converted %s → %s (%d bytes)\n", sourceName, convertOutput, len(pdfBytes))
		return nil
	},
}

func init() {
	pdfConvertCmd.Flags().StringVarP(&convertOutput, "output", "o", "", "Output PDF path (default: stdout)")
	pdfConvertCmd.Flags().StringVarP(&convertFormat, "format", "f", "", "Input format: md, html, docx, csv, json, yaml, xml, rst, ipynb, png, jpg, gif, webp, txt (default: auto)")
	pdfConvertCmd.Flags().BoolVar(&convertStdin, "stdin", false, "Read input from stdin instead of a file")
	pdfConvertCmd.Flags().StringVar(&convertStyle, "style", "", `Per-document style override as JSON, e.g. '{"paragraph":{"fontSize":14}}'`)
}
