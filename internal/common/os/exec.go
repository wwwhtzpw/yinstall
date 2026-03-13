// exec.go - 命令执行公共函数
// 提供智能用户切换的命令执行逻辑

package os

import (
	"fmt"
	"strings"

	"github.com/yinstall/internal/runner"
)

// ExecuteAsUser 以指定用户身份执行命令
// 自动判断当前用户，如果已经是目标用户则直接执行，否则使用 su 切换
//
// 参数：
//   - executor: 命令执行器
//   - targetUser: 目标用户名
//   - command: 要执行的命令
//   - showOutput: 是否显示输出
//
// 返回：
//   - 执行结果和错误
func ExecuteAsUser(executor runner.Executor, targetUser string, command string, showOutput bool) (runner.ExecResult, error) {
	// 获取当前执行用户
	result, _ := executor.Execute("whoami", false)
	currentUser := strings.TrimSpace(result.GetStdout())

	var cmd string
	if currentUser == targetUser {
		// 当前用户就是目标用户，直接执行
		cmd = command
	} else {
		// 需要切换用户，使用 su
		// 注意：命令中的单引号需要转义
		escapedCmd := strings.ReplaceAll(command, "'", "'\"'\"'")
		cmd = fmt.Sprintf("su - %s -c '%s'", targetUser, escapedCmd)
	}

	return executor.Execute(cmd, showOutput)
}

// ExecuteAsUserWithCheck 以指定用户身份执行命令（带错误检查）
// 自动判断当前用户，如果已经是目标用户则直接执行，否则使用 su 切换
// 如果命令执行失败（退出码非0），返回错误
//
// 参数：
//   - executor: 命令执行器
//   - targetUser: 目标用户名
//   - command: 要执行的命令
//   - showOutput: 是否显示输出
//
// 返回：
//   - 执行结果和错误
func ExecuteAsUserWithCheck(executor runner.Executor, targetUser string, command string, showOutput bool) (runner.ExecResult, error) {
	result, err := ExecuteAsUser(executor, targetUser, command, showOutput)
	if err != nil {
		return result, err
	}
	if result.GetExitCode() != 0 {
		return result, fmt.Errorf("command failed with exit code %d: %s", result.GetExitCode(), result.GetStderr())
	}
	return result, nil
}

// ExecuteAsUserWithEnv 以指定用户身份执行命令（带环境变量加载）
// 自动判断当前用户，如果已经是目标用户则直接执行，否则使用 su 切换
// 执行前会先 source 指定的环境变量文件
//
// 参数：
//   - executor: 命令执行器
//   - targetUser: 目标用户名
//   - envFile: 环境变量文件路径（如 /home/yashan/.bashrc）
//   - command: 要执行的命令
//   - showOutput: 是否显示输出
//
// 返回：
//   - 执行结果和错误
func ExecuteAsUserWithEnv(executor runner.Executor, targetUser string, envFile string, command string, showOutput bool) (runner.ExecResult, error) {
	// 获取当前执行用户
	result, _ := executor.Execute("whoami", false)
	currentUser := strings.TrimSpace(result.GetStdout())

	var cmd string
	if currentUser == targetUser {
		// 当前用户就是目标用户，直接执行（带环境变量加载）
		cmd = fmt.Sprintf("source %s && %s", envFile, command)
	} else {
		// 需要切换用户，使用 su（带环境变量加载）
		// 注意：整个命令需要用单引号包裹，内部的单引号需要转义
		fullCmd := fmt.Sprintf("source %s && %s", envFile, command)
		escapedCmd := strings.ReplaceAll(fullCmd, "'", "'\"'\"'")
		cmd = fmt.Sprintf("su - %s -c '%s'", targetUser, escapedCmd)
	}

	return executor.Execute(cmd, showOutput)
}

// ExecuteAsUserWithEnvCheck 以指定用户身份执行命令（带环境变量加载和错误检查）
// 自动判断当前用户，如果已经是目标用户则直接执行，否则使用 su 切换
// 执行前会先 source 指定的环境变量文件
// 如果命令执行失败（退出码非0），返回错误
//
// 参数：
//   - executor: 命令执行器
//   - targetUser: 目标用户名
//   - envFile: 环境变量文件路径（如 /home/yashan/.bashrc）
//   - command: 要执行的命令
//   - showOutput: 是否显示输出
//
// 返回：
//   - 执行结果和错误
func ExecuteAsUserWithEnvCheck(executor runner.Executor, targetUser string, envFile string, command string, showOutput bool) (runner.ExecResult, error) {
	result, err := ExecuteAsUserWithEnv(executor, targetUser, envFile, command, showOutput)
	if err != nil {
		return result, err
	}
	if result.GetExitCode() != 0 {
		return result, fmt.Errorf("command failed with exit code %d: %s", result.GetExitCode(), result.GetStderr())
	}
	return result, nil
}

// ExecuteAsUserWithEnvCheckCtx 以指定用户身份执行命令（带环境变量加载、错误检查和日志记录）
// 使用 StepContext 确保所有命令、输出都被记录到 DEBUG 日志
//
// 参数：
//   - ctx: 步骤上下文（包含 Logger）
//   - targetUser: 目标用户名
//   - envFile: 环境变量文件路径（如 /home/yashan/.bashrc）
//   - command: 要执行的命令
//   - showOutput: 是否显示输出
//
// 返回：
//   - 执行结果和错误
func ExecuteAsUserWithEnvCheckCtx(ctx *runner.StepContext, targetUser string, envFile string, command string, showOutput bool) (runner.ExecResult, error) {
	// 获取当前执行用户
	result, _ := ctx.Executor.Execute("whoami", false)
	currentUser := strings.TrimSpace(result.GetStdout())

	var cmd string
	var originalCommand string
	if currentUser == targetUser {
		// 当前用户就是目标用户，直接执行（带环境变量加载）
		cmd = fmt.Sprintf("source %s && %s", envFile, command)
		originalCommand = fmt.Sprintf("source %s && %s", envFile, command)
	} else {
		// 需要切换用户，使用 su（带环境变量加载）
		// 注意：整个命令需要用单引号包裹，内部的单引号需要转义
		fullCmd := fmt.Sprintf("source %s && %s", envFile, command)
		escapedCmd := strings.ReplaceAll(fullCmd, "'", "'\"'\"'")
		cmd = fmt.Sprintf("su - %s -c '%s'", targetUser, escapedCmd)
		originalCommand = fmt.Sprintf("su - %s -c 'source %s && %s'", targetUser, envFile, command)
	}

	// 执行命令
	result, err := ctx.Executor.Execute(cmd, false)

	// 记录命令执行结果到日志（无论成功失败）
	// 注意：即使 err != nil，result 也可能不为 nil（包含执行结果）
	if ctx.Logger != nil {
		if result != nil {
			ctx.Logger.LogCommand(
				ctx.Executor.Host(),
				ctx.CurrentStepID,
				originalCommand,
				result.GetStdout(),
				result.GetStderr(),
				result.GetExitCode(),
				result.GetDuration(),
			)
		} else if err != nil {
			// 如果 result 为 nil 但有错误，也记录错误
			ctx.Logger.LogCommand(
				ctx.Executor.Host(),
				ctx.CurrentStepID,
				originalCommand,
				"",
				fmt.Sprintf("executor returned error: %v", err),
				-1,
				0,
			)
		}
	}

	if err != nil {
		return result, err
	}

	// 检查退出码
	if result != nil && result.GetExitCode() != 0 {
		errMsg := result.GetStderr()
		if errMsg == "" {
			errMsg = result.GetStdout()
		}
		if errMsg == "" {
			errMsg = fmt.Sprintf("exit code %d", result.GetExitCode())
		}
		// 输出详细的错误信息到终端（包含命令、stdout、stderr、退出码）
		if ctx.Logger != nil {
			ctx.Logger.LogErrorExit(
				ctx.Executor.Host(),
				ctx.CurrentStepID,
				"",
				originalCommand,
				result.GetStdout(),
				result.GetStderr(),
				result.GetExitCode(),
				errMsg,
			)
		}
		return result, fmt.Errorf("command failed with exit code %d: %s", result.GetExitCode(), strings.TrimSpace(errMsg))
	}

	return result, nil
}

// ExecuteAsUserWithEnvCtx 以指定用户身份执行命令（带环境变量加载和日志记录）
// 使用 StepContext 确保所有命令、输出都被记录到 DEBUG 日志
//
// 参数：
//   - ctx: 步骤上下文（包含 Logger）
//   - targetUser: 目标用户名
//   - envFile: 环境变量文件路径（如 /home/yashan/.bashrc）
//   - command: 要执行的命令
//   - showOutput: 是否显示输出
//
// 返回：
//   - 执行结果和错误
func ExecuteAsUserWithEnvCtx(ctx *runner.StepContext, targetUser string, envFile string, command string, showOutput bool) (runner.ExecResult, error) {
	// 获取当前执行用户
	result, _ := ctx.Executor.Execute("whoami", false)
	currentUser := strings.TrimSpace(result.GetStdout())

	var cmd string
	var originalCommand string
	if currentUser == targetUser {
		// 当前用户就是目标用户，直接执行（带环境变量加载）
		cmd = fmt.Sprintf("source %s && %s", envFile, command)
		originalCommand = fmt.Sprintf("source %s && %s", envFile, command)
	} else {
		// 需要切换用户，使用 su（带环境变量加载）
		// 注意：整个命令需要用单引号包裹，内部的单引号需要转义
		fullCmd := fmt.Sprintf("source %s && %s", envFile, command)
		escapedCmd := strings.ReplaceAll(fullCmd, "'", "'\"'\"'")
		cmd = fmt.Sprintf("su - %s -c '%s'", targetUser, escapedCmd)
		originalCommand = fmt.Sprintf("su - %s -c 'source %s && %s'", targetUser, envFile, command)
	}

	// 执行命令
	result, err := ctx.Executor.Execute(cmd, false)

	// 记录命令执行结果到日志（无论成功失败）
	// 注意：即使 err != nil，result 也可能不为 nil（包含执行结果）
	if ctx.Logger != nil {
		if result != nil {
			ctx.Logger.LogCommand(
				ctx.Executor.Host(),
				ctx.CurrentStepID,
				originalCommand,
				result.GetStdout(),
				result.GetStderr(),
				result.GetExitCode(),
				result.GetDuration(),
			)
		} else if err != nil {
			// 如果 result 为 nil 但有错误，也记录错误
			ctx.Logger.LogCommand(
				ctx.Executor.Host(),
				ctx.CurrentStepID,
				originalCommand,
				"",
				fmt.Sprintf("executor returned error: %v", err),
				-1,
				0,
			)
		}
	}

	return result, err
}

// GetCurrentUser 获取当前执行用户
func GetCurrentUser(executor runner.Executor) (string, error) {
	result, err := executor.Execute("whoami", false)
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}
	if result.GetExitCode() != 0 {
		return "", fmt.Errorf("failed to get current user: exit code %d", result.GetExitCode())
	}
	return strings.TrimSpace(result.GetStdout()), nil
}
