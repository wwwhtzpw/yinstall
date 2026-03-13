package clean

import (
	"fmt"
	"strings"
	"time"

	"github.com/yinstall/internal/runner"
)

// StepCleanYCM Clean YCM installation
func StepCleanYCM() *runner.Step {
	return &runner.Step{
		ID:          "CLEAN-YCM",
		Name:        "Clean YCM",
		Description: "Clean YCM installation by stopping processes and removing directories",
		Tags:        []string{"clean", "ycm"},
		Optional:    false,

		PreCheck: func(ctx *runner.StepContext) error {
			ycmHome := ctx.GetParamString("ycm_home", "/opt/ycm")

			ctx.Logger.Info("YCM cleanup parameters:")
			ctx.Logger.Info("  YCM_HOME: %s", ycmHome)

			// Check if directory exists
			ctx.Logger.Info("Checking if YCM_HOME directory exists...")
			result, _ := ctx.Execute(fmt.Sprintf("test -d %s", ycmHome), false)
			if result == nil || result.GetExitCode() != 0 {
				ctx.Logger.Info("YCM_HOME directory does not exist (%s), skipping cleanup", ycmHome)
				return fmt.Errorf("skip: YCM_HOME directory does not exist")
			}
			ctx.Logger.Info("[OK] YCM_HOME directory exists")

			return nil
		},

		Action: func(ctx *runner.StepContext) error {
			ycmHome := ctx.GetParamString("ycm_home", "")

			ctx.Logger.Info("Starting YCM cleanup process")

			// 1. Find all YCM processes
			ctx.Logger.Info("Step 1: Finding YCM processes")
			// 在路径后添加 / 以避免误匹配（如 /opt/ycm 不会匹配到 /opt/ycm2）
			ycmHomePattern := ycmHome
			if !strings.HasSuffix(ycmHomePattern, "/") {
				ycmHomePattern = ycmHomePattern + "/"
			}
			findProcessCmd := fmt.Sprintf("ps -ef | grep '%s' | grep -v grep | awk '{print $2}'", ycmHomePattern)
			result, _ := ctx.Execute(findProcessCmd, false)

			var pids []string
			if result != nil && result.GetStdout() != "" {
				pids = strings.Split(strings.TrimSpace(result.GetStdout()), "\n")
				ctx.Logger.Info("Found %d processes to stop", len(pids))
				for _, pid := range pids {
					if strings.TrimSpace(pid) != "" {
						ctx.Logger.Info("  PID: %s", pid)
					}
				}
			} else {
				ctx.Logger.Info("No YCM processes found")
			}

			// 2. Stop processes gracefully (SIGTERM)
			if len(pids) > 0 {
				ctx.Logger.Info("Step 2: Stopping processes gracefully (SIGTERM)")
				for _, pid := range pids {
					pid = strings.TrimSpace(pid)
					if pid != "" {
						ctx.Logger.Info("Sending SIGTERM to PID %s", pid)
						ctx.Execute(fmt.Sprintf("kill -15 %s 2>/dev/null", pid), false)
					}
				}

				// Wait for processes to stop
				ctx.Logger.Info("Waiting 5 seconds for processes to stop...")
				time.Sleep(5 * time.Second)

				// 3. Force kill remaining processes (SIGKILL)
				ctx.Logger.Info("Step 3: Force killing remaining processes (SIGKILL)")
				result, _ = ctx.Execute(findProcessCmd, false)
				if result != nil && result.GetStdout() != "" {
					remainingPids := strings.Split(strings.TrimSpace(result.GetStdout()), "\n")
					for _, pid := range remainingPids {
						pid = strings.TrimSpace(pid)
						if pid != "" {
							ctx.Logger.Info("Force killing PID %s", pid)
							ctx.Execute(fmt.Sprintf("kill -9 %s 2>/dev/null", pid), false)
						}
					}
					time.Sleep(2 * time.Second)
				} else {
					ctx.Logger.Info("All processes stopped gracefully")
				}
			}

			// 4. Remove directory
			ctx.Logger.Info("Step 4: Removing YCM directory")
			ctx.Logger.Info("Removing YCM_HOME: %s", ycmHome)
			result, err := ctx.Execute(fmt.Sprintf("rm -rf %s", ycmHome), true)
			if err != nil || (result != nil && result.GetExitCode() != 0) {
				ctx.Logger.Warn("Failed to remove YCM_HOME: %v", err)
			} else {
				ctx.Logger.Info("YCM_HOME removed successfully")
			}

			ctx.Logger.Info("YCM cleanup completed")
			return nil
		},

		PostCheck: func(ctx *runner.StepContext) error {
			ycmHome := ctx.GetParamString("ycm_home", "")

			ctx.Logger.Info("Verifying cleanup results")

			// 1. Check if processes still exist
			// 在路径后添加 / 以避免误匹配
			ycmHomePattern := ycmHome
			if !strings.HasSuffix(ycmHomePattern, "/") {
				ycmHomePattern = ycmHomePattern + "/"
			}
			findProcessCmd := fmt.Sprintf("ps -ef | grep '%s' | grep -v grep", ycmHomePattern)
			result, _ := ctx.Execute(findProcessCmd, false)

			if result != nil && result.GetStdout() != "" {
				ctx.Logger.Error("WARNING: Some processes are still running:")
				ctx.Logger.Error("%s", result.GetStdout())
				return fmt.Errorf("failed to stop all YCM processes")
			} else {
				ctx.Logger.Info("[OK] All processes stopped successfully")
			}

			// 2. Check if directory still exists
			result, _ = ctx.Execute(fmt.Sprintf("test -d %s", ycmHome), false)
			if result != nil && result.GetExitCode() == 0 {
				ctx.Logger.Warn("WARNING: YCM_HOME still exists: %s", ycmHome)
			} else {
				ctx.Logger.Info("[OK] YCM_HOME removed successfully")
			}

			ctx.Logger.Info("Cleanup verification completed")
			return nil
		},
	}
}
