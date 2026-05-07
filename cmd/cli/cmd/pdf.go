// Copyright 2026 Anna Veretennykova
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import "github.com/spf13/cobra"

var pdfCmd = &cobra.Command{
	Use:   "pdf",
	Short: "PDF conversion tools",
	Long:  "Convert documents to PDF and serve the conversion API.",
}

func init() {
	pdfCmd.AddCommand(pdfConvertCmd)
	pdfCmd.AddCommand(pdfServeCmd)
}
