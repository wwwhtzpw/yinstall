package db

import (
	"fmt"
	"strings"

	commonos "github.com/yinstall/internal/common/os"
	"github.com/yinstall/internal/runner"
)

// StepC006ACleanStaleBashrc removes stale source lines from .bashrc
// that reference non-existent files, preventing errors when running
// `su - yashan` in subsequent steps.
func StepC006ACleanStaleBashrc() *runner.Step {
	return &runner.Step{
		ID:          "C-006A",
		Name:        "Clean Stale Bashrc Entries",
		Description: "Remove stale environment entries from .bashrc before installation",
		Tags:        []string{"db", "env"},
		Optional:    true,

		PreCheck: func(ctx *runner.StepContext) error {
			user := ctx.GetParamString("os_user", "yashan")
			for _, th := range ctx.HostsToRun() {
				hctx := ctx.ForHost(th)
				_, err := commonos.GetUserHomeDir(hctx.Executor, user)
				if err != nil {
					return err
				}
			}
			return nil
		},

		Action: func(ctx *runner.StepContext) error {
			user := ctx.GetParamString("os_user", "yashan")

			for _, th := range ctx.HostsToRun() {
				hctx := ctx.ForHost(th)
				homeDir, err := commonos.GetUserHomeDir(hctx.Executor, user)
				if err != nil {
					continue
				}

				bashrc := fmt.Sprintf("%s/.bashrc", homeDir)
				r, _ := hctx.Execute(fmt.Sprintf("test -f %s", bashrc), false)
				if r == nil || r.GetExitCode() != 0 {
					continue
				}

				// Find all `source ...yasboot...bashrc` lines pointing to missing files
				cmd := fmt.Sprintf("grep -n 'source.*\\.yasboot/.*_yasdb_home/conf/.*\\.bashrc' %s 2>/dev/null", bashrc)
				result, _ := hctx.Execute(cmd, false)
				if result == nil || result.GetStdout() == "" {
					hctx.Logger.Info("No yasboot source entries in .bashrc on %s", th.Host)
					continue
				}

				lines := strings.Split(strings.TrimSpace(result.GetStdout()), "\n")
				cleaned := 0
				for _, line := range lines {
					// Extract the file path from `source /path/to/file`
					parts := strings.SplitN(line, "source ", 2)
					if len(parts) < 2 {
						continue
					}
					sourcePath := strings.TrimSpace(parts[1])

					// Check if the referenced file exists
					checkResult, _ := hctx.Execute(fmt.Sprintf("test -f %s", sourcePath), false)
					if checkResult != nil && checkResult.GetExitCode() == 0 {
						continue
					}

					hctx.Logger.Info("Removing stale entry on %s: source %s (file not found)", th.Host, sourcePath)
					// Use escaped path for sed pattern
					escapedPath := strings.ReplaceAll(sourcePath, "/", "\\/")
					sedCmd := fmt.Sprintf("sed -i '/source.*%s/d' %s", escapedPath, bashrc)
					hctx.Execute(sedCmd, false)
					cleaned++
				}

				// Also clean stale YASCS_HOME if the path doesn't exist
				cmd = fmt.Sprintf("grep -oP 'export YASCS_HOME=\\K.*' %s 2>/dev/null", bashrc)
				result, _ = hctx.Execute(cmd, false)
				if result != nil && result.GetStdout() != "" {
					yascsPath := strings.TrimSpace(result.GetStdout())
					checkResult, _ := hctx.Execute(fmt.Sprintf("test -d %s", yascsPath), false)
					if checkResult == nil || checkResult.GetExitCode() != 0 {
						hctx.Logger.Info("Removing stale YASCS_HOME on %s: %s (dir not found)", th.Host, yascsPath)
						commonos.BashrcRemoveLine(hctx.Executor, bashrc, "export YASCS_HOME=")
						cleaned++
					}
				}

				if cleaned > 0 {
					hctx.Logger.Info("Cleaned %d stale entries from .bashrc on %s", cleaned, th.Host)
				} else {
					hctx.Logger.Info("No stale entries found in .bashrc on %s", th.Host)
				}
			}
			return nil
		},
	}
}
