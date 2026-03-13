package ssh

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewExecutorWithFallback 测试fallback逻辑
func TestNewExecutorWithFallback(t *testing.T) {
	tests := []struct {
		name           string
		host           string
		keyPathExists  bool
		defaultPwd     string
		expectError    bool
		errorContains  string
	}{
		{
			name:          "local host should use LocalExecutor",
			host:          "localhost",
			expectError:   false,
		},
		{
			name:          "127.0.0.1 should use LocalExecutor",
			host:          "127.0.0.1",
			expectError:   false,
		},
		{
			name:          "invalid remote host without credentials",
			host:          "invalid.host.example.com",
			keyPathExists: false,
			defaultPwd:    "",
			expectError:   true,
			errorContains: "all authentication methods failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Host:       tt.host,
				Port:       22,
				User:       "root",
				AuthMethod: "auto",
				Timeout:    0,
			}

			executor, err := NewExecutorWithFallback(cfg, tt.defaultPwd)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("error message should contain '%s', got: %s", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if executor == nil {
					t.Errorf("executor should not be nil")
				}
			}
		})
	}
}

// TestNewExecutorWithFallbackKeyPath 测试密钥路径处理
func TestNewExecutorWithFallbackKeyPath(t *testing.T) {
	// 创建临时目录和密钥文件
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "id_rsa")

	// 创建一个假的密钥文件
	if err := os.WriteFile(keyPath, []byte("fake key"), 0600); err != nil {
		t.Fatalf("failed to create test key file: %v", err)
	}

	cfg := Config{
		Host:       "localhost",
		Port:       22,
		User:       "root",
		AuthMethod: "auto",
		KeyPath:    keyPath,
		Timeout:    0,
	}

	executor, err := NewExecutorWithFallback(cfg, "")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if executor == nil {
		t.Errorf("executor should not be nil")
	}
}

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
