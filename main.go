package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/scaleoutllc/terrallel/internal/cli"
	"github.com/spf13/cobra"
)

func main() {
	var manifestPath string
	var dryRun bool
	var rootCmd = &cobra.Command{
		Use:   "terrallel",
		Short: "run terraform in parallel across dependent workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			dashIndex := cmd.ArgsLenAtDash()
			if len(args) == 0 || dashIndex == 0 {
				return errors.New("no target specified")
			}
			if dashIndex == -1 || strings.TrimSpace(strings.Join(args[dashIndex:], "")) == "" {
				return errors.New("no terraform command defined after `--`")
			}
			return cli.Root(manifestPath, args[0], args[dashIndex:], dryRun)
		},
	}
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	rootCmd.SetUsageTemplate(`Usage:
  terrallel [-cd] <target> -- <terraform-command>

Flags:
{{.Flags.FlagUsages | trimTrailingWhitespaces}}

Example:
  terrallel network -- init
  terrallel network -- apply -auto-approve
  terrallel network -- destroy -auto-approve`)
	rootCmd.Flags().StringVarP(&manifestPath, "manifest", "m", "Infrafile", "Path to the manifest file")
	rootCmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Enable dry-run mode")
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(errorBar(err))
		os.Exit(1)
	}
}

func errorBar(err error) string {
	prefix := color.RedString("│ ")
	lines := strings.Split(err.Error(), "\n")
	mainErr := color.RedString("Error: ") + color.New(color.FgHiWhite, color.Bold).Sprint(lines[0])
	if len(lines) > 1 {
		lines = append([]string{mainErr, ""}, lines[1:]...)
	} else {
		lines = []string{mainErr}
	}
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return color.RedString("╷\n") + strings.Join(lines, "\n") + color.RedString("\n╵")
}
