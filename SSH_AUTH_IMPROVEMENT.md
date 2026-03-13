# SSH 认证改进说明

## 问题分析

原始错误信息：
```
Error: connectivity check failed for 10.10.10.135: failed to connect to 10.10.10.135:22: ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain
```

**问题根源：**
- 用户没有提供 `--ssh-password` 参数
- 代码直接尝试密码认证，但密码为空
- 没有尝试免密登陆（SSH密钥）
- 错误信息不清晰，用户不知道该如何解决

## 解决方案

### 1. 新增 `NewExecutorWithFallback` 函数

位置：`internal/ssh/executor.go`

**功能：** 实现多层次认证降级机制

**认证优先级：**
1. **用户明确指定的认证方式** - 如果用户提供了 `--ssh-password` 或 `--ssh-key-path`，直接使用
2. **SSH密钥认证（免密登陆）** - 自动尝试 `~/.ssh/id_rsa` 或用户指定的密钥
3. **默认密码** - 如果提供了默认密码，尝试使用
4. **详细错误提示** - 所有方式都失败时，给出清晰的错误信息

### 2. 改进的错误信息

新的错误信息格式：
```
failed to connect to 10.10.10.135:22: all authentication methods failed
  - Tried: SSH key-based authentication (if key exists)
  - Tried: default password authentication
  Please provide valid credentials using --ssh-password or --ssh-key-path
```

**优势：**
- 清晰说明尝试了哪些认证方式
- 明确告诉用户如何解决问题
- 用户知道需要提供 `--ssh-password` 或 `--ssh-key-path`

### 3. 修改的文件

#### `internal/ssh/executor.go`
- 添加 `NewExecutorWithFallback()` 函数
- 添加 `path/filepath` 导入

#### `internal/cli/os.go`
- 修改 `createExecutor()` 函数，使用 `NewExecutorWithFallback`
- 当用户没有提供密码时，自动尝试免密登陆

#### `internal/cli/standby.go`
- 修改 `createPrimaryExecutor()` 函数，使用 `NewExecutorWithFallback`

#### `internal/cli/clean.go`
- 修改 executor 创建逻辑，使用 `NewExecutorWithFallback`

#### `internal/ssh/executor_test.go`
- 新增单元测试，验证 fallback 逻辑

## 使用示例

### 场景1：使用SSH密钥（推荐）

```bash
# 如果本地有 ~/.ssh/id_rsa，会自动尝试免密登陆
yinstall db --targets 10.10.10.135 --skip-os

# 或明确指定密钥路径
yinstall db --targets 10.10.10.135 --ssh-key-path /path/to/key --skip-os
```

### 场景2：使用密码认证

```bash
# 明确提供密码
yinstall db --targets 10.10.10.135 --ssh-password 'your-password' --skip-os
```

### 场景3：混合场景

```bash
# 先尝试免密，失败后提示用户提供密码
yinstall db --targets 10.10.10.135 --skip-os
# 如果免密失败，会显示：
# Error: failed to connect to 10.10.10.135:22: all authentication methods failed
#   - Tried: SSH key-based authentication (if key exists)
#   - Tried: default password authentication
#   Please provide valid credentials using --ssh-password or --ssh-key-path
```

## 技术细节

### 认证流程图

```
用户执行命令
    ↓
检查是否本机执行 → 是 → 使用 LocalExecutor
    ↓ 否
用户是否明确指定认证方式？
    ↓ 是 → 直接使用指定的认证方式
    ↓ 否
尝试 SSH 密钥认证
    ↓
密钥文件存在？
    ↓ 是 → 尝试连接
    ↓ 否 → 跳过
    ↓
连接成功？
    ↓ 是 → 返回 executor
    ↓ 否 → 继续
    ↓
尝试默认密码认证
    ↓
默认密码非空？
    ↓ 是 → 尝试连接
    ↓ 否 → 跳过
    ↓
连接成功？
    ↓ 是 → 返回 executor
    ↓ 否 → 返回详细错误信息
```

### 代码示例

```go
// 在 CLI 中使用 fallback 逻辑
if flags.SSHPassword == "" && flags.SSHAuth == "password" {
    return ssh.NewExecutorWithFallback(cfg, defaultSSHPassword())
}
return ssh.NewExecutor(cfg)
```

## 测试

运行单元测试验证 fallback 逻辑：

```bash
go test ./internal/ssh -v
```

测试覆盖：
- ✓ 本机执行（localhost）
- ✓ 本机执行（127.0.0.1）
- ✓ 远程主机无凭证时的错误处理
- ✓ 密钥路径处理

## 向后兼容性

- ✓ 现有的 `NewExecutor()` 函数保持不变
- ✓ 用户明确指定认证方式时行为不变
- ✓ 只在用户未提供密码时启用 fallback 逻辑
- ✓ 所有现有命令行参数继续有效

## 后续改进建议

1. **支持环境变量** - 从 `YASINSTALL_SSH_PASSWORD` 环境变量读取默认密码
2. **配置文件支持** - 从 `~/.yinstall/config` 读取默认认证配置
3. **交互式密码输入** - 当所有自动认证都失败时，提示用户交互式输入密码
4. **SSH Agent 支持** - 优先使用 SSH Agent 中的密钥
5. **连接重试** - 添加可配置的重试机制
