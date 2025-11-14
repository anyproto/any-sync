package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	rootPath string
	topN     int
	since    string
	spaceID  string
)

var rootCmd = &cobra.Command{
	Use:   "spacetop",
	Short: "Interactive TUI for analyzing spacestore databases",
	Long: `Spacetop is an interactive TUI tool for analyzing spacestore databases.
It shows the top N trees by number of changes across all spaces, with filtering
options by time and space ID. Navigate with arrow keys and view object details.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAnalyzer()
	},
}

func init() {
	rootCmd.Flags().StringVarP(&rootPath, "path", "p", "", "Root path containing space databases (required)")
	rootCmd.Flags().IntVarP(&topN, "top", "n", 20, "Number of top trees to show")
	rootCmd.Flags().StringVarP(&since, "since", "s", "", "Filter by last change time (e.g., 10m, 1h, 24h)")
	rootCmd.Flags().StringVar(&spaceID, "space-id", "", "Filter by specific space ID")

	rootCmd.MarkFlagRequired("path")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
