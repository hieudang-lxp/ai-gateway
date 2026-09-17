package aictl

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	var address string
	var timeout time.Duration
	var asJSON bool
	root := &cobra.Command{Use: "aictl", Short: "Inspect Claude Code, Codex and Cursor usage from your gateway", SilenceUsage: true, SilenceErrors: true}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().StringVar(&address, "url", "http://localhost:8788", "Local gateway origin")
	root.PersistentFlags().DurationVar(&timeout, "timeout", 15*time.Second, "HTTP request timeout")
	root.PersistentFlags().BoolVar(&asJSON, "json", false, "Write machine-readable JSON to stdout")
	client := func() (*Client, error) { return newClient(address, timeout) }

	status := &cobra.Command{Use: "status", Short: "Show gateway health and last sync for all collectors", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := client()
		if err != nil {
			return err
		}
		if err = c.health(cmd.Context()); err != nil {
			return err
		}
		s, err := c.summary(cmd.Context(), "period=month")
		if err != nil {
			return err
		}
		if asJSON {
			return writeJSON(cmd.OutOrStdout(), map[string]any{"gateway": "ok", "sources": s.Sources, "pricing": s.Pricing})
		}
		return writeStatus(cmd.OutOrStdout(), s)
	}}
	doctor := &cobra.Command{Use: "doctor", Short: "Diagnose collector sync and pricing with suggested fixes", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := client()
		if err != nil {
			return err
		}
		checks := []Check{}
		if err = c.health(cmd.Context()); err != nil {
			checks = append(checks, Check{"gateway", "error", err.Error(), "Start docker compose up -d; check the published port and --url."})
		} else {
			checks = append(checks, Check{"gateway", "ok", "HTTP service is reachable", ""})
			s, fetchErr := c.summary(cmd.Context(), "period=month")
			if fetchErr != nil {
				checks = append(checks, Check{"usage API", "error", fetchErr.Error(), "Run the local gateway with collection enabled (-collect=true)."})
			} else {
				checks = append(checks, diagnose(s, time.Now())...)
			}
		}
		if asJSON {
			err = writeJSON(cmd.OutOrStdout(), checks)
		} else {
			err = writeChecks(cmd.OutOrStdout(), checks)
		}
		if err != nil {
			return err
		}
		for _, check := range checks {
			if check.State != "ok" {
				return fmt.Errorf("doctor found issues; see checks above")
			}
		}
		return nil
	}}
	root.AddCommand(status, doctor)
	for _, name := range []string{"usage", "export"} {
		var month bool
		var days int
		var format string
		command := &cobra.Command{Use: name, Args: cobra.NoArgs}
		if name == "usage" {
			command.Short = "Show usage totals by source and model (current month by default)"
		} else {
			command.Short = "Export model usage aggregates to stdout (CSV by default)"
			command.Flags().StringVar(&format, "format", "csv", "Export format: csv or json")
		}
		command.Flags().BoolVar(&month, "month", false, "Current calendar month in the gateway timezone (default)")
		command.Flags().IntVar(&days, "days", 30, "Rolling calendar days, or 0 for all history")
		command.MarkFlagsMutuallyExclusive("month", "days")
		command.RunE = func(cmd *cobra.Command, _ []string) error {
			query := "period=month"
			if cmd.Flags().Changed("days") {
				if days < 0 || days > 3650 {
					return fmt.Errorf("--days must be between 0 and 3650")
				}
				query = "days=" + strconv.Itoa(days)
			}
			if cmd.Name() == "export" && format != "csv" && format != "json" {
				return fmt.Errorf("--format must be csv or json")
			}
			if cmd.Name() == "export" && asJSON && cmd.Flags().Changed("format") && format != "json" {
				return fmt.Errorf("choose --json or --format csv, not both")
			}
			c, err := client()
			if err != nil {
				return err
			}
			s, err := c.summary(cmd.Context(), query)
			if err != nil {
				return err
			}
			if asJSON || format == "json" {
				return writeJSON(cmd.OutOrStdout(), s)
			}
			if cmd.Name() == "export" {
				return writeCSV(cmd.OutOrStdout(), s)
			}
			return writeUsage(cmd.OutOrStdout(), s)
		}
		root.AddCommand(command)
	}
	return root
}
