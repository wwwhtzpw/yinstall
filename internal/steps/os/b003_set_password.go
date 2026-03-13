package os

import (
	"fmt"

	"github.com/yinstall/internal/runner"
)

// StepB003SetUserPassword Set user password (optional)
func StepB003SetUserPassword() *runner.Step {
	return &runner.Step{
		ID:          "B-003",
		Name:        "Set User Password",
		Description: "Set product user password",
		Tags:        []string{"os", "user"},
		Optional:    true,

		PreCheck: func(ctx *runner.StepContext) error {
			password := ctx.GetParamString("os_user_password", "")
			if password == "" {
				return fmt.Errorf("password not provided")
			}
			return nil
		},

		Action: func(ctx *runner.StepContext) error {
			user := ctx.GetParamString("os_user", "yashan")
			password := ctx.GetParamString("os_user_password", "")

			cmd := fmt.Sprintf("echo '%s' | passwd %s --stdin", password, user)
			if _, err := ctx.ExecuteWithCheck(cmd, true); err != nil {
				return fmt.Errorf("failed to set password for %s: %w", user, err)
			}
			return nil
		},
	}
}
