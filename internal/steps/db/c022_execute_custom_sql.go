package db

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yinstall/internal/common/file"
	"github.com/yinstall/internal/runner"
)

// StepC022ExecuteCustomSQL Execute custom SQL script
func StepC022ExecuteCustomSQL() *runner.Step {
	return &runner.Step{
		ID:          "C-022",
		Name:        "Execute Custom SQL Script",
		Description: "Execute custom SQL script using yasql",
		Tags:        []string{"db", "sql", "custom"},
		Optional:    true,

		PreCheck: func(ctx *runner.StepContext) error {
			sqlScript := ctx.GetParamString("db_custom_sql_script", "")
			if sqlScript == "" {
				return fmt.Errorf("no custom SQL script specified, skipping")
			}
			return nil
		},

		Action: func(ctx *runner.StepContext) error {
			sqlScript := ctx.GetParamString("db_custom_sql_script", "")
			installPath := ctx.GetParamString("db_install_path", "/data/yashan/yasdb_home")
			sysPassword := ctx.GetParamString("db_admin_password", "")
			clusterName := ctx.GetParamString("db_cluster_name", "yashandb")
			beginPort := ctx.GetParamInt("db_begin_port", 1688)

			if sysPassword == "" {
				return fmt.Errorf("db_admin_password is required for SQL execution")
			}

			// 解析脚本路径（支持 remote:, local:, r:, l: 前缀）
			remotePath, err := resolveScriptPath(ctx, sqlScript)
			if err != nil {
				return fmt.Errorf("failed to resolve SQL script path: %w", err)
			}

			ctx.Logger.Info("Executing custom SQL script: %s", remotePath)

			// 构建 yasql 命令
			yasqlPath := filepath.Join(installPath, "bin/yasql")

			// yasql 连接命令：yasql sys/password@localhost:port/yasdb -f script.sql
			connectStr := fmt.Sprintf("sys/%s@localhost:%d/%s", sysPassword, beginPort, clusterName)
			cmd := fmt.Sprintf("%s %s -f %s", yasqlPath, connectStr, remotePath)

			ctx.Logger.Info("Running yasql command...")
			result, err := ctx.Execute(cmd, false)
			if err != nil {
				return fmt.Errorf("failed to execute yasql: %w", err)
			}

			// 检查执行结果
			if result.GetExitCode() != 0 {
				ctx.Logger.Error("SQL script execution failed with exit code: %d", result.GetExitCode())
				ctx.Logger.Error("STDOUT: %s", result.GetStdout())
				ctx.Logger.Error("STDERR: %s", result.GetStderr())
				return fmt.Errorf("SQL script execution failed")
			}

			// 检查输出中是否有 YAS-NNNNN 错误代码
			output := result.GetStdout() + result.GetStderr()
			if hasYasError(output) {
				ctx.Logger.Error("SQL script execution completed with YAS errors:")
				ctx.Logger.Error("Output: %s", output)
				return fmt.Errorf("SQL script execution failed with YAS errors")
			}

			ctx.Logger.Info("Custom SQL script executed successfully")
			ctx.Logger.Info("Output: %s", result.GetStdout())
			return nil
		},

		PostCheck: func(ctx *runner.StepContext) error {
			return nil
		},
	}
}

// resolveScriptPath 解析脚本路径，支持多种格式
// 格式：
//   - remote:/path/to/script.sql 或 r:/path/to/script.sql - 远程路径
//   - local:/path/to/script.sql 或 l:/path/to/script.sql - 本地路径
//   - /absolute/path/script.sql - 绝对路径（优先远程，不存在则上传）
//   - relative/path/script.sql - 相对路径（从本地软件目录查找）
func resolveScriptPath(ctx *runner.StepContext, scriptPath string) (string, error) {
	scriptPath = strings.TrimSpace(scriptPath)
	if scriptPath == "" {
		return "", fmt.Errorf("empty script path")
	}

	// 处理前缀方式
	if strings.HasPrefix(scriptPath, "remote:") || strings.HasPrefix(scriptPath, "r:") {
		// 明确指定远程路径
		remotePath := strings.TrimPrefix(scriptPath, "remote:")
		remotePath = strings.TrimPrefix(remotePath, "r:")
		remotePath = strings.TrimSpace(remotePath)

		// 验证远程文件是否存在
		if !file.FileExists(ctx.Executor, remotePath) {
			return "", fmt.Errorf("remote file not found: %s", remotePath)
		}
		ctx.Logger.Info("Using remote SQL script: %s", remotePath)
		return remotePath, nil
	}

	if strings.HasPrefix(scriptPath, "local:") || strings.HasPrefix(scriptPath, "l:") {
		// 明确指定本地路径
		localPath := strings.TrimPrefix(scriptPath, "local:")
		localPath = strings.TrimPrefix(localPath, "l:")
		localPath = strings.TrimSpace(localPath)

		// 使用 FindAndDistribute 上传文件
		remotePath, err := file.FindAndDistribute(
			ctx.Executor,
			localPath,
			ctx.LocalSoftwareDirs,
			ctx.RemoteSoftwareDir,
		)
		if err != nil {
			return "", fmt.Errorf("failed to upload local file: %w", err)
		}
		ctx.Logger.Info("Uploaded local SQL script to: %s", remotePath)
		return remotePath, nil
	}

	// 处理绝对路径
	if filepath.IsAbs(scriptPath) {
		// 先检查远程是否存在
		if file.FileExists(ctx.Executor, scriptPath) {
			ctx.Logger.Info("Using existing remote SQL script: %s", scriptPath)
			return scriptPath, nil
		}

		// 远程不存在，尝试从本地上传
		ctx.Logger.Info("Remote file not found, trying to upload from local...")
		remotePath, err := file.FindAndDistribute(
			ctx.Executor,
			scriptPath,
			ctx.LocalSoftwareDirs,
			ctx.RemoteSoftwareDir,
		)
		if err != nil {
			return "", fmt.Errorf("file not found on remote or local: %s", scriptPath)
		}
		ctx.Logger.Info("Uploaded SQL script to: %s", remotePath)
		return remotePath, nil
	}

	// 处理相对路径 - 从本地软件目录查找
	ctx.Logger.Info("Relative path detected, searching in local software directories...")
	remotePath, err := file.FindAndDistribute(
		ctx.Executor,
		scriptPath,
		ctx.LocalSoftwareDirs,
		ctx.RemoteSoftwareDir,
	)
	if err != nil {
		return "", fmt.Errorf("failed to find and upload SQL script: %w", err)
	}
	ctx.Logger.Info("Uploaded SQL script to: %s", remotePath)
	return remotePath, nil
}

// hasYasError 检查输出中是否包含 YAS-NNNNN 格式的错误代码
func hasYasError(output string) bool {
	// 匹配 YAS-NNNNN 格式（N 为数字）
	re := regexp.MustCompile(`YAS-\d{5}`)
	return re.MatchString(output)
}
