// primary_env.go - 主库环境变量文件路径辅助函数
// 提供获取主库环境变量文件路径的通用逻辑

package standby

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yinstall/internal/runner"
)

// GetPrimaryEnvFile 获取主库环境变量文件路径
// 优先级：
// 1. 如果指定了 primary_env_file 参数，使用指定的路径（绝对路径或相对用户家目录）
// 2. 自动检测：优先使用 ~/.yasboot/<cluster>_yasdb_home/conf/<cluster>.bashrc
// 3. 如果不存在，使用 ~/.bashrc（单实例）或 ~/.<port>（多实例）
func GetPrimaryEnvFile(ctx *runner.StepContext, executor runner.Executor) (string, error) {
	// 1. 如果指定了 primary_env_file 参数，使用指定的路径
	specifiedEnvFile := ctx.GetParamString("primary_env_file", "")
	if specifiedEnvFile != "" {
		// 如果是绝对路径，直接使用
		if strings.HasPrefix(specifiedEnvFile, "/") {
			// 检查文件是否存在
			result, _ := executor.Execute(fmt.Sprintf("test -f %s", specifiedEnvFile), false)
			if result != nil && result.GetExitCode() == 0 {
				return specifiedEnvFile, nil
			}
			return "", fmt.Errorf("specified primary environment file %s not found", specifiedEnvFile)
		}
		// 如果是相对路径（如 .bashrc 或 .1688），需要拼接用户家目录
		primaryUser := ctx.GetParamString("primary_os_user", "yashan")
		homeDir, err := getUserHomeDir(executor, primaryUser)
		if err != nil {
			return "", fmt.Errorf("failed to get home directory for primary user %s: %w", primaryUser, err)
		}
		envFile := filepath.Join(homeDir, specifiedEnvFile)
		// 检查文件是否存在
		result, _ := executor.Execute(fmt.Sprintf("test -f %s", envFile), false)
		if result != nil && result.GetExitCode() == 0 {
			return envFile, nil
		}
		return "", fmt.Errorf("specified primary environment file %s not found", envFile)
	}

	// 2. 自动检测：优先使用 ~/.yasboot/<cluster>_yasdb_home/conf/<cluster>.bashrc
	clusterName := ctx.GetParamString("db_cluster_name", "yashandb")
	primaryUser := ctx.GetParamString("primary_os_user", "yashan")
	homeDir, err := getUserHomeDir(executor, primaryUser)
	if err != nil {
		return "", fmt.Errorf("failed to get home directory for primary user %s: %w", primaryUser, err)
	}

	// 优先使用 .yasboot 目录下的环境变量文件
	yasbootEnvFile := fmt.Sprintf("%s/.yasboot/%s_yasdb_home/conf/%s.bashrc", homeDir, clusterName, clusterName)
	result, _ := executor.Execute(fmt.Sprintf("test -f %s", yasbootEnvFile), false)
	if result != nil && result.GetExitCode() == 0 {
		return yasbootEnvFile, nil
	}

	// 3. 如果不存在，使用 ~/.bashrc 或 ~/.<port>
	beginPort := ctx.GetParamInt("db_begin_port", 1688)
	// 检查是否有多个 yasdb 进程（多实例场景）
	result, _ = executor.Execute("pgrep -c -x yasdb 2>/dev/null || echo 0", false)
	yasdbCount := 0
	if result != nil && result.GetStdout() != "" {
		fmt.Sscanf(strings.TrimSpace(result.GetStdout()), "%d", &yasdbCount)
	}

	// 优先检查 ~/.<port> 文件是否存在（即使只有一个 yasdb 进程，也可能使用端口号文件）
	portEnvFile := fmt.Sprintf("%s/.%d", homeDir, beginPort)
	result, _ = executor.Execute(fmt.Sprintf("test -f %s", portEnvFile), false)
	if result != nil && result.GetExitCode() == 0 {
		return portEnvFile, nil
	}

	// 如果端口号文件不存在，根据 yasdb 进程数选择
	var envFile string
	if yasdbCount > 1 {
		// 多实例场景：使用 ~/.<port>
		envFile = fmt.Sprintf("%s/.%d", homeDir, beginPort)
	} else {
		// 单实例场景：使用 ~/.bashrc
		envFile = fmt.Sprintf("%s/.bashrc", homeDir)
	}

	// 检查文件是否存在
	result, _ = executor.Execute(fmt.Sprintf("test -f %s", envFile), false)
	if result != nil && result.GetExitCode() == 0 {
		return envFile, nil
	}

	return "", fmt.Errorf("primary environment file not found (tried: %s, %s, %s)", yasbootEnvFile, portEnvFile, envFile)
}

// GetPrimaryOSUser 获取主库数据库用户
func GetPrimaryOSUser(ctx *runner.StepContext) string {
	return ctx.GetParamString("primary_os_user", "yashan")
}

// getUserHomeDir 获取用户主目录（内部辅助函数）
func getUserHomeDir(executor runner.Executor, user string) (string, error) {
	result, _ := executor.Execute(fmt.Sprintf("getent passwd %s | cut -d: -f6", user), false)
	if result == nil || result.GetStdout() == "" {
		return "", fmt.Errorf("cannot determine home directory for user %s", user)
	}
	homeDir := strings.TrimSpace(result.GetStdout())
	if homeDir == "" {
		homeDir = fmt.Sprintf("/home/%s", user)
	}
	return homeDir, nil
}

// ExtractClusterNameFromEnvFile 从环境文件中提取集群名
// 环境文件格式示例：
//
//	source /home/yashan/.yasboot/huang_yasdb_home/conf/huang.bashrc
//
// 从路径中提取集群名（如 huang）
func ExtractClusterNameFromEnvFile(executor runner.Executor, envFile string) (string, error) {
	// 读取环境文件内容
	result, err := executor.Execute(fmt.Sprintf("cat %s", envFile), false)
	if err != nil {
		return "", fmt.Errorf("failed to read environment file %s: %w", envFile, err)
	}
	if result == nil || result.GetStdout() == "" {
		return "", fmt.Errorf("environment file %s is empty", envFile)
	}

	content := result.GetStdout()
	lines := strings.Split(content, "\n")

	// 查找包含 source 和 .yasboot 的行
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "source") && strings.Contains(line, ".yasboot") {
			// 提取路径，格式：source /path/to/.yasboot/<cluster>_yasdb_home/conf/<cluster>.bashrc
			// 或者：source ~/.yasboot/<cluster>_yasdb_home/conf/<cluster>.bashrc
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			path := parts[1]

			// 处理 ~ 符号（如果需要，可以在这里展开 ~ 路径）
			// 目前直接使用路径，因为 executor.Execute 会处理 ~ 符号

			// 从路径中提取集群名
			// 路径格式：.../.yasboot/<cluster>_yasdb_home/conf/<cluster>.bashrc
			// 或者：.../<cluster>_yasdb_home/conf/<cluster>.bashrc
			if strings.Contains(path, "_yasdb_home/conf/") {
				// 提取 <cluster>_yasdb_home 部分
				startIdx := strings.Index(path, "_yasdb_home/conf/")
				if startIdx > 0 {
					// 向前查找，找到集群名的开始位置
					// 查找最后一个 / 或 ~/ 或 .yasboot/ 之后的位置
					prefix := path[:startIdx]
					// 查找最后一个 / 的位置
					lastSlash := strings.LastIndex(prefix, "/")
					if lastSlash >= 0 {
						clusterName := prefix[lastSlash+1:]
						if clusterName != "" {
							return clusterName, nil
						}
					}
				}
			}

			// 备用方法：从文件名提取（如 huang.bashrc）
			if strings.Contains(path, ".bashrc") {
				// 提取文件名（不含路径）
				lastSlash := strings.LastIndex(path, "/")
				if lastSlash >= 0 {
					filename := path[lastSlash+1:]
					// 移除 .bashrc 后缀
					if strings.HasSuffix(filename, ".bashrc") {
						clusterName := strings.TrimSuffix(filename, ".bashrc")
						if clusterName != "" {
							return clusterName, nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("cannot extract cluster name from environment file %s (no source line with .yasboot found)", envFile)
}
