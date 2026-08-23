// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/annavetech/annave-pdf-engine-golang/internal/engine"
)

var rootCmd = &cobra.Command{
	Use:   "annave",
	Short: "ANNAVE developer tools",
	Long:  "ANNAVE — developer tools by Anna Veretennykova (www.annave.tech)",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ANNAVE PDF Engine v%s\n", engine.EngineVersion)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(pdfCmd)
}
