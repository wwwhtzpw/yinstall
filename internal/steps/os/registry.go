package os

import (
	"github.com/yinstall/internal/runner"
)

// GetAllSteps Get all OS baseline steps
func GetAllSteps() []*runner.Step {
	return []*runner.Step{
		StepB000CheckConnectivity(),
		StepB001CreateGroup(),
		StepB002CreateUser(),
		StepB003SetUserPassword(),
		StepB004ConfigureSudoers(),
		StepB028ConfigureUmask(),
		StepB005SetTimezone(),
		StepB006WriteSysctlConfig(),
		StepB007ApplySysctl(),
		StepB008WriteLimitsConfig(),
		StepB029ConfigureHugepages(),
		StepB009WriteKernelArgs(),
		StepB010MountISO(),
		StepB011WriteYumRepo(),
		StepB012InstallDeps(),
		StepB013ConfigureChrony(),
		StepB014DisableFirewall(),
		StepB015OpenFirewallPorts(),
		StepB016RebootCheck(),
		// Local disk setup
		StepB025SetupLocalDisk(),
		// YAC auto-discover shared disks (runs before B-026 when diskgroups not configured)
		StepB026AAutoDiscoverSharedDisks(),
		// YAC diskgroup validation (must run before multipath steps)
		// B-026 检测到非多路径磁盘时会自动设置 yac_multipath_enable=true
		StepB026ValidateYACDiskgroups(),
		// Hostname configuration
		StepB027SetHostname(),
		// YAC multipath related (runs only when needed, auto-enabled by B-026)
		StepB017InstallMultipath(),
		StepB018CollectWWID(),
		StepB019WriteMultipathConf(),
		StepB020EnableMultipathd(),
		StepB021VerifyMultipath(),
		StepB022WriteUdevRules(),
		StepB023TriggerUdev(),
		StepB024VerifyDiskPermissions(),
	}
}
