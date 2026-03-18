// env.go - 环境变量配置公共函数
// 提供环境变量配置的通用逻辑，被 DB 安装和备库添加步骤共用

package os

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yinstall/internal/runner"
)

// EnvConfig 环境变量配置参数
type EnvConfig struct {
	User        string // 操作系统用户名
	ClusterName string // 数据库集群名
	DataPath    string // 数据目录路径
	BeginPort   int    // 数据库起始端口
	IsYACMode   bool   // 是否 YAC 模式
}

// EnvResult 环境变量配置结果
type EnvResult struct {
	HomeDir       string // 用户主目录
	TargetEnvFile string // 目标环境变量文件
	YasdbCount    int    // 运行中的 yasdb 进程数（保留兼容，不再用于判断文件路径）
	BashrcPath    string // 生成的 bashrc 路径
}

// GetUserHomeDir 获取用户主目录
func GetUserHomeDir(executor runner.Executor, user string) (string, error) {
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

// GetYasdbProcessCount 获取运行中的 yasdb 进程数
func GetYasdbProcessCount(executor runner.Executor) int {
	result, _ := executor.Execute("pgrep -c -x yasdb 2>/dev/null || echo 0", false)
	yasdbCount := 0
	if result != nil && result.GetStdout() != "" {
		fmt.Sscanf(strings.TrimSpace(result.GetStdout()), "%d", &yasdbCount)
	}
	return yasdbCount
}

// DetermineEnvFile 根据端口号确定环境变量文件路径
// 规则：默认端口 1688 写入 ~/.bashrc；其他端口写入 ~/.port<端口号>
func DetermineEnvFile(homeDir string, beginPort int) string {
	if beginPort == 1688 {
		return filepath.Join(homeDir, ".bashrc")
	}
	return filepath.Join(homeDir, fmt.Sprintf(".port%d", beginPort))
}

// GetBashrcPath 获取 yasboot 生成的 bashrc 文件路径
func GetBashrcPath(homeDir, clusterName string) string {
	return fmt.Sprintf("%s/.yasboot/%s_yasdb_home/conf/%s.bashrc", homeDir, clusterName, clusterName)
}

// bashrcReplaceLine 在文件中查找匹配 grepPattern 的行：
//   - 如果找到且内容不同，用 sed 替换为 newLine
//   - 如果未找到，追加 newLine
//   - 如果已完全相同，不做任何操作
//
// 返回 "added" / "updated" / "unchanged"
func bashrcReplaceLine(executor runner.Executor, file, grepPattern, newLine string) string {
	// 精确匹配
	exactCmd := fmt.Sprintf("grep -qxF '%s' %s 2>/dev/null", newLine, file)
	r, _ := executor.Execute(exactCmd, false)
	if r != nil && r.GetExitCode() == 0 {
		return "unchanged"
	}

	// 模式匹配（旧记录）
	patternCmd := fmt.Sprintf("grep -qE '%s' %s 2>/dev/null", grepPattern, file)
	r, _ = executor.Execute(patternCmd, false)
	if r != nil && r.GetExitCode() == 0 {
		// 用 | 作 sed 分隔符，避免模式中的 / 破坏语法
		sedCmd := fmt.Sprintf("sed -i '\\|%s|c\\%s' %s", grepPattern, newLine, file)
		executor.Execute(sedCmd, false)
		return "updated"
	}

	appendCmd := fmt.Sprintf("echo '%s' >> %s", newLine, file)
	executor.Execute(appendCmd, false)
	return "added"
}

// BashrcRemoveLine 从文件中删除匹配 grepPattern 的行
func BashrcRemoveLine(executor runner.Executor, file, grepPattern string) bool {
	checkCmd := fmt.Sprintf("grep -qE '%s' %s 2>/dev/null", grepPattern, file)
	r, _ := executor.Execute(checkCmd, false)
	if r == nil || r.GetExitCode() != 0 {
		return false
	}
	// 用 | 作 sed 分隔符，避免模式中的 / 破坏语法
	sedCmd := fmt.Sprintf("sed -i '\\|%s|d' %s", grepPattern, file)
	executor.Execute(sedCmd, false)
	return true
}

// ConfigureEnvVars 配置环境变量（幂等：已存在的条目会被更新而非重复追加）
//
// 规则：
//   - 端口 1688（默认）：将 source <clusterName>.bashrc 写入 ~/.bashrc
//   - 其他端口：将所有内容写入 ~/.port<port>，不修改 ~/.bashrc
//
// YAC 模式下每个节点均需调用本函数。
func ConfigureEnvVars(executor runner.Executor, cfg *EnvConfig) (*EnvResult, error) {
	homeDir, err := GetUserHomeDir(executor, cfg.User)
	if err != nil {
		return nil, err
	}

	yasdbCount := GetYasdbProcessCount(executor)
	targetEnvFile := DetermineEnvFile(homeDir, cfg.BeginPort)
	bashrcPath := GetBashrcPath(homeDir, cfg.ClusterName)

	result := &EnvResult{
		HomeDir:       homeDir,
		TargetEnvFile: targetEnvFile,
		YasdbCount:    yasdbCount,
		BashrcPath:    bashrcPath,
	}

	// 检查 yasboot 生成的 bashrc 是否存在
	checkResult, _ := executor.Execute(fmt.Sprintf("test -f %s", bashrcPath), false)
	if checkResult == nil || checkResult.GetExitCode() != 0 {
		return result, fmt.Errorf("generated bashrc not found at %s", bashrcPath)
	}

	// 非默认端口场景下创建目标文件（~/.port<port>）
	if cfg.BeginPort != 1688 {
		checkResult, _ = executor.Execute(fmt.Sprintf("test -f %s", targetEnvFile), false)
		if checkResult == nil || checkResult.GetExitCode() != 0 {
			cmd := fmt.Sprintf("touch %s && chown %s:%s %s", targetEnvFile, cfg.User, cfg.User, targetEnvFile)
			if _, err := executor.Execute(cmd, true); err != nil {
				return result, fmt.Errorf("failed to create env file %s: %w", targetEnvFile, err)
			}
		}
	}

	if cfg.BeginPort == 1688 {
		// === 默认端口 1688：修改 ~/.bashrc ===

		// 1. yasboot completion（只添加，不替换）
		completionPath := fmt.Sprintf("%s/.yasboot/yasboot.completion.bash", homeDir)
		completionLine := fmt.Sprintf("[ -f %s ] && source %s", completionPath, completionPath)
		bashrcReplaceLine(executor, targetEnvFile,
			"yasboot\\.completion\\.bash", completionLine)

		// 2. source {clusterName}_yasdb_home/conf/{clusterName}.bashrc
		// 匹配任意集群名的 source 行，存在旧集群名则替换
		sourceLine := fmt.Sprintf("source %s", bashrcPath)
		bashrcReplaceLine(executor, targetEnvFile,
			"source.*\\.yasboot/.*_yasdb_home/conf/.*\\.bashrc", sourceLine)

		// 3. YAC 模式：YASCS_HOME（写入 .bashrc）
		if cfg.IsYACMode {
			instanceResult, _ := executor.Execute(fmt.Sprintf("ls %s/ycs/ 2>/dev/null | head -1", cfg.DataPath), false)
			if instanceResult != nil && instanceResult.GetStdout() != "" {
				instanceName := strings.TrimSpace(instanceResult.GetStdout())
				yascsHome := fmt.Sprintf("%s/ycs/%s", cfg.DataPath, instanceName)
				exportLine := fmt.Sprintf("export YASCS_HOME=%s", yascsHome)
				bashrcReplaceLine(executor, targetEnvFile,
					"export YASCS_HOME=", exportLine)
			}
		}
	} else {
		// === 非默认端口：写入 ~/.port<port>，不动 ~/.bashrc ===

		// 1. source {clusterName}.bashrc
		sourceLine := fmt.Sprintf("source %s", bashrcPath)
		bashrcReplaceLine(executor, targetEnvFile,
			"source.*\\.yasboot/.*_yasdb_home/conf/.*\\.bashrc", sourceLine)

		// 2. YAC 模式：YASCS_HOME
		if cfg.IsYACMode {
			instanceResult, _ := executor.Execute(fmt.Sprintf("ls %s/ycs/ 2>/dev/null | head -1", cfg.DataPath), false)
			if instanceResult != nil && instanceResult.GetStdout() != "" {
				instanceName := strings.TrimSpace(instanceResult.GetStdout())
				yascsHome := fmt.Sprintf("%s/ycs/%s", cfg.DataPath, instanceName)
				exportLine := fmt.Sprintf("export YASCS_HOME=%s", yascsHome)
				bashrcReplaceLine(executor, targetEnvFile,
					"export YASCS_HOME=", exportLine)
			}
		}
		// 注意：非默认端口不向 ~/.bashrc 写入任何内容
	}

	return result, nil
}

// CleanEnvVars 清理指定集群的环境变量条目
// - 端口 1688：从 ~/.bashrc 中删除对应条目
// - 其他端口：删除整个 ~/.port<port> 文件
// YAC 模式下需在每个节点分别调用
func CleanEnvVars(executor runner.Executor, user, clusterName, dataPath string, beginPort int) error {
	homeDir, err := GetUserHomeDir(executor, user)
	if err != nil {
		return err
	}

	if beginPort == 0 {
		beginPort = 1688
	}

	if beginPort == 1688 {
		// 默认端口：从 ~/.bashrc 中精确删除该集群的条目
		bashrc := filepath.Join(homeDir, ".bashrc")

		r, _ := executor.Execute(fmt.Sprintf("test -f %s", bashrc), false)
		if r == nil || r.GetExitCode() != 0 {
			return nil
		}

		// 1. 删除 source {clusterName}_yasdb_home/conf/{clusterName}.bashrc
		clusterSourcePattern := fmt.Sprintf("source.*\\.yasboot/%s_yasdb_home/conf/%s\\.bashrc", clusterName, clusterName)
		BashrcRemoveLine(executor, bashrc, clusterSourcePattern)

		// 2. 删除 export YASCS_HOME={dataPath}/ycs/
		if dataPath != "" {
			yascsPattern := fmt.Sprintf("export YASCS_HOME=%s/ycs/", dataPath)
			BashrcRemoveLine(executor, bashrc, yascsPattern)
		}

		// 3. 如果 .bashrc 中没有其他集群的 source 行，也删除 yasboot completion
		otherClusterCmd := fmt.Sprintf("grep -cE 'source.*\\.yasboot/.*_yasdb_home/conf/.*\\.bashrc' %s 2>/dev/null || echo 0", bashrc)
		countResult, _ := executor.Execute(otherClusterCmd, false)
		remaining := 0
		if countResult != nil {
			fmt.Sscanf(strings.TrimSpace(countResult.GetStdout()), "%d", &remaining)
		}
		if remaining == 0 {
			BashrcRemoveLine(executor, bashrc, "yasboot\\.completion\\.bash")
		}

		// 4. 清理可能遗留的空行
		executor.Execute(fmt.Sprintf("sed -i '/^$/N;/^\\n$/d' %s", bashrc), false)
	} else {
		// 非默认端口：直接删除整个 ~/.port<port> 文件
		portFile := filepath.Join(homeDir, fmt.Sprintf(".port%d", beginPort))
		r, _ := executor.Execute(fmt.Sprintf("test -f %s", portFile), false)
		if r != nil && r.GetExitCode() == 0 {
			executor.Execute(fmt.Sprintf("rm -f %s", portFile), true)
		}
	}

	return nil
}

// VerifyYasboot 验证 yasboot 是否可用
func VerifyYasboot(executor runner.Executor, user string) (string, bool) {
	cmd := fmt.Sprintf("su - %s -c 'which yasboot 2>/dev/null'", user)
	result, _ := executor.Execute(cmd, false)
	if result != nil && result.GetExitCode() == 0 {
		return strings.TrimSpace(result.GetStdout()), true
	}
	return "", false
} 