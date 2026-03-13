// registry.go - 备库扩容步骤注册
// 本文件注册所有备库扩容相关的步骤

package standby

import "github.com/yinstall/internal/runner"

// GetAllSteps 返回所有备库扩容步骤
func GetAllSteps() []*runner.Step {
	return []*runner.Step{
		// 主库检查
		StepE000CheckPrimaryConnectivity(),
		StepE001CheckPrimaryStatus(),
		StepE002CheckArchiveMode(),      // 检查归档模式
		StepE003CheckReplicationAddr(),  // 检查 replication_addr 参数

		// 备库检查
		StepE004CheckStandbyConnectivity(),
		StepE005CheckArchiveDest(),        // 检查归档路径是否已包含目标端
		StepE006CheckNetworkConnectivity(),
		StepE007CheckAndCleanupExistingNodes(), // 检查并清理已存在的节点

		// 扩容执行（在主库执行）
		StepE008GenExpansionConfig(),
		StepE009InstallSoftware(),
		StepE010AddStandbyInstance(),
		StepE011CheckSyncStatus(),

		// 备库后续配置
		StepE012ConfigEnvVars(),
		StepE013ConfigAutostart(),
		StepE014VerifyExpansion(),

		// 清理步骤（危险操作）
		StepE015CleanupFailedExpansion(),

		// 最后一步：显示集群状态
		StepE016ShowClusterStatus(),
	}
}
