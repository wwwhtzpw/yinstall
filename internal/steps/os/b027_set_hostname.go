package os

import (
	"fmt"
	"strings"

	commonos "github.com/yinstall/internal/common/os"
	"github.com/yinstall/internal/runner"
)

// StepB027SetHostname Configure system hostname
func StepB027SetHostname() *runner.Step {
	return &runner.Step{
		ID:          "B-027",
		Name:        "Set Hostname",
		Description: "Configure system hostname on all nodes",
		Tags:        []string{"os", "hostname"},
		Optional:    true,
		Global:      true,

		PreCheck: func(ctx *runner.StepContext) error {
			hostnameParam := ctx.GetParamString("os_hostname", "")
			hostnames := parseHostnames(hostnameParam)
			targetCount := len(ctx.HostsToRun())

			if targetCount > 1 {
				if len(hostnames) > 1 && len(hostnames) != targetCount {
					return fmt.Errorf("hostname count (%d) does not match node count (%d), provide 1 prefix or %d hostnames",
						len(hostnames), targetCount, targetCount)
				}
			}
			return nil
		},

		Action: func(ctx *runner.StepContext) error {
			hostnameParam := ctx.GetParamString("os_hostname", "")
			hostnames := parseHostnames(hostnameParam)
			hosts := ctx.HostsToRun()

			type nodeInfo struct {
				ip       string
				hostname string
			}
			var nodes []nodeInfo

			for i, th := range hosts {
				var newHostname string
				if len(hosts) > 1 {
					if len(hostnames) == 0 {
						newHostname = fmt.Sprintf("yashandb%02d", i+1)
					} else if len(hostnames) == 1 {
						newHostname = fmt.Sprintf("%s%02d", hostnames[0], i+1)
					} else {
						newHostname = hostnames[i]
					}
				} else {
					if len(hostnames) == 0 {
						newHostname = "yashandb"
					} else {
						newHostname = hostnames[0]
					}
				}

				ctx.Logger.Info("[%s] Setting hostname to: %s", th.Host, newHostname)

				cmd := fmt.Sprintf("hostnamectl set-hostname %s", newHostname)
				result, err := th.Executor.Execute(cmd, true)
				if err != nil {
					return fmt.Errorf("[%s] failed to set hostname: %w", th.Host, err)
				}
				if result != nil && result.GetExitCode() != 0 {
					return fmt.Errorf("[%s] hostnamectl failed: %s", th.Host, result.GetStderr())
				}

				nodes = append(nodes, nodeInfo{ip: th.Host, hostname: newHostname})
				ctx.Logger.Info("[%s] Hostname set to: %s", th.Host, newHostname)
			}

			if len(nodes) > 0 {
				var entries []string
				for _, n := range nodes {
					entries = append(entries, fmt.Sprintf("%s  %s", n.ip, n.hostname))
				}
				ctx.Logger.Info("Writing hostname entries to /etc/hosts on all nodes: %v", entries)
				for _, th := range hosts {
					if err := commonos.UpdateManagedHostsBlock(th.Executor, entries); err != nil {
						return fmt.Errorf("[%s] failed to update /etc/hosts: %w", th.Host, err)
					}
				}
			}

			return nil
		},

		PostCheck: func(ctx *runner.StepContext) error {
			hosts := ctx.HostsToRun()
			for _, th := range hosts {
				result, err := th.Executor.Execute("hostname", false)
				if err != nil {
					return fmt.Errorf("[%s] failed to verify hostname: %w", th.Host, err)
				}
				if result != nil {
					ctx.Logger.Info("[%s] Current hostname: %s", th.Host, strings.TrimSpace(result.GetStdout()))
				}
			}
			return nil
		},
	}
}

// parseHostnames Parse comma-separated hostname string
func parseHostnames(hostnameParam string) []string {
	if hostnameParam == "" {
		return []string{}
	}

	parts := strings.Split(hostnameParam, ",")
	var hostnames []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			hostnames = append(hostnames, trimmed)
		}
	}
	return hostnames
}
