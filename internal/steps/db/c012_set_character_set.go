package db

import (
	"fmt"
	"path/filepath"

	"github.com/yinstall/internal/runner"
)

// StepC006SetCharacterSet Set character set in cluster config
func StepC012SetCharacterSet() *runner.Step {
	return &runner.Step{
		ID:          "C-012",
		Name:        "Set Character Set",
		Description: "Configure database character set",
		Tags:        []string{"db", "config"},
		Optional:    true,

		PreCheck: func(ctx *runner.StepContext) error {
			charset := ctx.GetParamString("db_character_set", "utf8")
			if charset != "utf8" && charset != "gbk" && charset != "gb18030" {
				return fmt.Errorf("unsupported character set: %s (supported: utf8, gbk, gb18030)", charset)
			}
			return nil
		},

		Action: func(ctx *runner.StepContext) error {
			charset := ctx.GetParamString("db_character_set", "utf8")
			stageDir := ctx.GetParamString("db_stage_dir", "/home/yashan/install")
			clusterName := ctx.GetParamString("db_cluster_name", "yashandb")

			// Default is utf8, skip if not changed
			if charset == "utf8" {
				ctx.Logger.Info("Character set is default (utf8), skipping modification")
				return nil
			}

			configPath := filepath.Join(stageDir, clusterName+".toml")

			ctx.Logger.Info("Setting character set to: %s", charset)

			// Modify config file
			cmd := fmt.Sprintf(`sed -i 's/CHARACTER_SET.*/CHARACTER_SET = "%s"/' %s`, charset, configPath)
			if _, err := ctx.ExecuteWithCheck(cmd, false); err != nil {
				return fmt.Errorf("failed to set character set: %w", err)
			}

			// Verify change
			result, _ := ctx.Execute(fmt.Sprintf("grep CHARACTER_SET %s", configPath), false)
			if result != nil {
				ctx.Logger.Info("Config updated: %s", result.GetStdout())
			}

			return nil
		},

		PostCheck: func(ctx *runner.StepContext) error {
			charset := ctx.GetParamString("db_character_set", "utf8")
			stageDir := ctx.GetParamString("db_stage_dir", "/home/yashan/install")
			clusterName := ctx.GetParamString("db_cluster_name", "yashandb")

			configPath := filepath.Join(stageDir, clusterName+".toml")

			result, _ := ctx.Execute(fmt.Sprintf("grep -i 'CHARACTER_SET.*%s' %s", charset, configPath), false)
			if result == nil || result.GetExitCode() != 0 {
				ctx.Logger.Warn("Could not verify character set setting")
			}

			return nil
		},
	}
}
