// h006_install_jdk.go - 安装 JDK
// H-006: 安装 JDK RPM 包（可选步骤）

package ymp

import (
	"fmt"
	"strings"

	commonfile "github.com/yinstall/internal/common/file"
	"github.com/yinstall/internal/runner"
)

// StepH006InstallJDK 安装 JDK
func StepH006InstallJDK() *runner.Step {
	return &runner.Step{
		ID:          "H-006",
		Name:        "Install JDK",
		Description: "Install JDK from RPM package (optional, skip if JDK already available)",
		Tags:        []string{"ymp", "jdk"},
		Optional:    true,

		PreCheck: func(ctx *runner.StepContext) error {
			jdkEnable := ctx.GetParamBool("ymp_jdk_enable", false)
			if !jdkEnable {
				return fmt.Errorf("JDK installation not enabled (--ymp-jdk-enable=false)")
			}

			// 如果 java 已存在，跳过安装
			result, _ := ctx.Execute("which java 2>/dev/null", false)
			if result != nil && result.GetExitCode() == 0 {
				vResult, _ := ctx.Execute("java -version 2>&1 | head -1", false)
				version := ""
				if vResult != nil {
					version = strings.TrimSpace(vResult.GetStdout())
					if version == "" {
						version = strings.TrimSpace(vResult.GetStderr())
					}
				}
				ctx.Logger.Info("JDK already installed: %s", version)
				return fmt.Errorf("JDK already installed: %s", version)
			}

			jdkPackage := ctx.GetParamString("ymp_jdk_package", "")
			if jdkPackage == "" {
				return fmt.Errorf("--ymp-jdk-package is required when --ymp-jdk-enable=true")
			}
			return nil
		},

		Action: func(ctx *runner.StepContext) error {
			jdkPackage := ctx.GetParamString("ymp_jdk_package", "")

			ctx.Logger.Info("Looking for JDK package: %s", jdkPackage)

			// 查找并分发 JDK 包
			fullPath, err := commonfile.FindAndDistribute(
				ctx.Executor,
				jdkPackage,
				ctx.LocalSoftwareDirs,
				ctx.RemoteSoftwareDir,
			)
			if err != nil {
				return fmt.Errorf("JDK package not found: %w", err)
			}

			ctx.Logger.Info("Installing JDK from: %s", fullPath)

			// 根据文件类型选择安装方式
			var cmd string
			if strings.HasSuffix(fullPath, ".rpm") {
				cmd = fmt.Sprintf("rpm -ivh %s", fullPath)
			} else if strings.HasSuffix(fullPath, ".tar.gz") || strings.HasSuffix(fullPath, ".tgz") {
				installDir := ctx.GetParamString("ymp_jdk_install_dir", "/usr/local")
				cmd = fmt.Sprintf("tar -zxf %s -C %s", fullPath, installDir)
			} else {
				return fmt.Errorf("unsupported JDK package format: %s (expected .rpm or .tar.gz)", fullPath)
			}

			if _, err := ctx.ExecuteWithCheck(cmd, true); err != nil {
				return fmt.Errorf("failed to install JDK: %w", err)
			}

			ctx.Logger.Info("JDK installed successfully")
			return nil
		},

		PostCheck: func(ctx *runner.StepContext) error {
			result, _ := ctx.Execute("java -version 2>&1 | head -1", false)
			if result == nil {
				return fmt.Errorf("java command not available after installation")
			}
			version := strings.TrimSpace(result.GetStdout())
			if version == "" {
				version = strings.TrimSpace(result.GetStderr())
			}
			if version == "" {
				return fmt.Errorf("java -version returned empty output")
			}
			ctx.Logger.Info("✓ JDK verified: %s", version)
			return nil
		},
	}
}
