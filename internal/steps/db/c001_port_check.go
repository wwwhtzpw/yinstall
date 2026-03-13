package db

import (
	"fmt"
	"strings"

	"github.com/yinstall/internal/runner"
)

// StepC000PortCheck 检查 --begin-port 对应端口在当前主机是否已被占用；占用则报错退出
func StepC001PortCheck() *runner.Step {
	return &runner.Step{
		ID:          "C-001",
		Name:        "Check Begin Port Available",
		Description: "Verify db begin port is not in use on the host",
		Tags:        []string{"db", "port", "precheck"},
		Optional:    false,

		PreCheck: func(ctx *runner.StepContext) error {
			return nil
		},

		Action: func(ctx *runner.StepContext) error {
			beginPort := ctx.GetParamInt("db_begin_port", 1688)
			installPath := ctx.GetParamString("db_install_path", "/data/yashan/yasdb_home")
			clusterName := ctx.GetParamString("db_cluster_name", "yashandb")

			for _, th := range ctx.HostsToRun() {
				hctx := ctx.ForHost(th)

				// 1. 检查端口是否被占用
				// 使用精确匹配避免误匹配（如 1688 不会匹配到 16888）
				hctx.Logger.Info("Checking if port %d is in use on %s...", beginPort, th.Host)
				portCmd := fmt.Sprintf("ss -tuln 2>/dev/null | grep -E ':%d([^0-9]|$)' || netstat -tlnp 2>/dev/null | grep -E ':%d([^0-9]|$)'", beginPort, beginPort)
				result, err := hctx.Executor.Execute(portCmd, false)
				if err != nil {
					return fmt.Errorf("failed to check port %d on %s: %w", beginPort, th.Host, err)
				}
				if result != nil && result.GetExitCode() == 0 && strings.TrimSpace(result.GetStdout()) != "" {
					portInfo := strings.TrimSpace(result.GetStdout())
					return fmt.Errorf("port %d is already in use on %s (--begin-port); port info: %s; please choose another port or stop the process using it", beginPort, th.Host, portInfo)
				}
				hctx.Logger.Info("✓ Port %d is available on %s", beginPort, th.Host)

				// 2. 检查安装目录是否存在（如果存在，说明数据库可能已安装）
				hctx.Logger.Info("Checking if installation directory exists on %s...", th.Host)
				dirCmd := fmt.Sprintf("test -d %s", installPath)
				dirResult, _ := hctx.Executor.Execute(dirCmd, false)
				if dirResult != nil && dirResult.GetExitCode() == 0 {
					// 检查目录是否为空（可能只是创建了目录但未安装）
					checkEmptyCmd := fmt.Sprintf("test -f %s/bin/yasboot || test -f %s/om/bin/monit", installPath, installPath)
					emptyResult, _ := hctx.Executor.Execute(checkEmptyCmd, false)
					if emptyResult != nil && emptyResult.GetExitCode() == 0 {
						// 目录存在且包含数据库文件，说明已安装
						return fmt.Errorf("database installation already exists on %s: directory %s exists and contains database files (cluster: %s, port: %d); use --force to delete and reinstall, or use clean command to remove it first", th.Host, installPath, clusterName, beginPort)
					}
					// 目录存在但为空，给出警告但不阻止（会在后续步骤处理）
					hctx.Logger.Warn("Installation directory %s exists but appears to be empty on %s", installPath, th.Host)
				} else {
					hctx.Logger.Info("✓ Installation directory %s does not exist on %s", installPath, th.Host)
				}

				// 3. 检查安装路径下是否有 yasdb 进程在运行
				// 注意：端口检查已经通过 ss/netstat 完成，这里只检查安装路径下的进程
				// 使用更精确的匹配：检查进程路径包含安装路径，且命令行参数包含集群名
				hctx.Logger.Info("Checking for running yasdb processes under %s on %s...", installPath, th.Host)
				// 在路径后添加 / 以避免误匹配（如 /data/1233 不会匹配到 /data/12334）
				installPathPattern := installPath
				if !strings.HasSuffix(installPathPattern, "/") {
					installPathPattern = installPathPattern + "/"
				}
				// 检查安装路径下是否有进程，且命令行参数中包含集群名（-c clusterName）
				processCmd := fmt.Sprintf("ps -ef | grep -E '(yasdb|yasagent|yasom)' | grep -v grep | grep '%s' | grep -E '(-c %s|--cluster %s)'", installPathPattern, clusterName, clusterName)
				processResult, _ := hctx.Executor.Execute(processCmd, false)
				if processResult != nil && processResult.GetExitCode() == 0 && strings.TrimSpace(processResult.GetStdout()) != "" {
					processInfo := strings.TrimSpace(processResult.GetStdout())
					return fmt.Errorf("YashanDB processes are running under %s on %s (cluster: %s, port: %d); process info: %s; please stop the database first or use clean command to remove it", installPath, th.Host, clusterName, beginPort, processInfo)
				}
				hctx.Logger.Info("✓ No conflicting yasdb processes found under %s on %s", installPath, th.Host)
			}
			return nil
		},

		PostCheck: func(ctx *runner.StepContext) error {
			return nil
		},
	}
}
