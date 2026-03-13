// registry.go - YCM 安装步骤注册表
// 返回所有 YCM 安装步骤，按执行顺序排列

package ycm

import "github.com/yinstall/internal/runner"

// GetAllSteps returns all YCM installation steps in execution order
func GetAllSteps() []*runner.Step {
	return []*runner.Step{
		// Dependencies
		StepG001InstallDeps(),

		// Package extraction and setup
		StepG002ExtractPackage(),
		StepG003SetOwnership(),

		// Configuration
		StepG004CheckDeployConfig(),
		StepG005ConfigurePorts(),

		// Port validation
		StepG006CheckPorts(),

		// Deployment
		StepG007Deploy(),

		// Verification
		StepG008VerifyProcess(),
		StepG009VerifyPorts(),
		StepG010VerifyWeb(),
	}
}
