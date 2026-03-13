// pkg.go - 软件包管理公共函数
// 提供包检测、安装命令构建等通用逻辑，被 OS 安装步骤和 YCM 安装步骤共用

package os

import (
	"fmt"
	"strings"

	"github.com/yinstall/internal/runner"
)

// IsPackageInstalled checks if a package is already installed via rpm or dpkg
func IsPackageInstalled(ctx *runner.StepContext, pkg, pkgManager string) bool {
	var checkCmd string
	if pkgManager == "apt" {
		checkCmd = fmt.Sprintf("dpkg -l %s 2>/dev/null | grep -q '^ii'", pkg)
	} else {
		checkCmd = fmt.Sprintf("rpm -q %s >/dev/null 2>&1", pkg)
	}

	result, _ := ctx.Execute(checkCmd, false)
	return result != nil && result.GetExitCode() == 0
}

// FilterUninstalledPackages returns only packages that are not yet installed
func FilterUninstalledPackages(ctx *runner.StepContext, packages, pkgManager string) []string {
	pkgList := strings.Fields(packages)
	var uninstalled []string

	for _, pkg := range pkgList {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" {
			continue
		}

		if !IsPackageInstalled(ctx, pkg, pkgManager) {
			uninstalled = append(uninstalled, pkg)
		} else {
			ctx.Logger.Info("  Package '%s' already installed", pkg)
		}
	}

	return uninstalled
}

// BuildInstallCmd builds the install command based on package manager and yum mode
// yumMode 取值: "local-iso"（使用本地 ISO 仓库）、"none"（使用默认/网络仓库）
func BuildInstallCmd(pkgManager, yumMode, packages string, isRHEL8 bool) string {
	if yumMode == "local-iso" {
		if isRHEL8 {
			return fmt.Sprintf("%s -y install --disablerepo=\\* --enablerepo=local-baseos --enablerepo=local-appstream %s", pkgManager, packages)
		}
		return fmt.Sprintf("%s -y install --disablerepo=\\* --enablerepo=local %s", pkgManager, packages)
	}

	if pkgManager == "apt" {
		return fmt.Sprintf("apt-get -y install %s", packages)
	}
	return fmt.Sprintf("%s -y install %s", pkgManager, packages)
}
