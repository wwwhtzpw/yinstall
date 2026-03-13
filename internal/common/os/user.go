// user.go - 用户管理公共函数
// 提供用户创建和删除的通用逻辑，被 OS 准备步骤共用

package os

import (
	"fmt"

	"github.com/yinstall/internal/runner"
)

// ForceDeleteUser kills all processes belonging to a user and deletes the user account.
// This is a common operation used during force-mode recreation of OS users and groups.
func ForceDeleteUser(ctx *runner.StepContext, user string) error {
	// Kill all processes owned by the user
	ctx.Execute(fmt.Sprintf("pkill -9 -u %s 2>/dev/null; sleep 1; pkill -9 -u %s 2>/dev/null || true", user, user), true)
	// Delete the user with fallback chain
	if _, err := ctx.ExecuteWithCheck(fmt.Sprintf("/usr/sbin/userdel -rf %s 2>/dev/null || /usr/sbin/userdel -f %s 2>/dev/null || /usr/sbin/userdel %s", user, user, user), true); err != nil {
		return fmt.Errorf("failed to delete user %s: %w", user, err)
	}
	return nil
}
