# 华为存储多路径磁盘支持说明

## 概述

已更新 yinstall 以支持华为存储多路径磁盘（/dev/ultrapath 开头）。华为磁盘已经是多路径设备，不需要通过 Linux multipath 软件进行配置，但需要通过 UDEV 规则配置磁盘权限。

## 主要变更

### 1. 磁盘检测函数 (`internal/common/os/disk.go`)

#### 新增函数：`IsHuaweiMultipathDisk(disk string) bool`
- **功能**：检查磁盘是否为华为存储多路径磁盘
- **判断条件**：磁盘路径以 `/dev/ultrapath` 开头
- **返回值**：true 表示华为磁盘，false 表示其他磁盘

```go
func IsHuaweiMultipathDisk(disk string) bool {
    return strings.HasPrefix(disk, "/dev/ultrapath")
}
```

#### 新增函数：`GetHuaweiDiskWWID(ctx *runner.StepContext, disk string) (string, error)`
- **功能**：获取华为存储多路径磁盘的 WWID
- **获取方法**（按优先级）：
  1. 使用 `udevadm info -a --name <disk>` 获取 `ATTR{wwid}`
  2. 使用 `udevadm info --query=property` 获取 `ID_WWN`
  3. 使用 `udevadm info --query=property` 获取 `ID_SERIAL`
- **命令示例**：
  ```bash
  udevadm info -a --name /dev/ultrapath/dg5 | grep ATTR{wwid}
  ```

#### 修改函数：`GetDiskWWID(ctx *runner.StepContext, disk string) (string, error)`
- **新增逻辑**：检测到华为磁盘时，调用 `GetHuaweiDiskWWID()` 获取 WWID
- **保持兼容**：非华为磁盘的处理逻辑保持不变

### 2. 多路径配置 (`internal/steps/os/b019_write_multipath_conf.go`)

#### 修改内容
- **跳过华为磁盘**：在 `collectDisksWWID` 函数中添加华为磁盘检测
- **检测逻辑**：
  ```go
  if commonos.IsHuaweiMultipathDisk(disk) {
      ctx.Logger.Info("  %s is Huawei multipath disk, skipping multipath configuration", disk)
      continue
  }
  ```
- **原因**：华为磁盘已经是多路径设备，不需要通过 multipath 软件配置

### 3. UDEV 规则配置 (`internal/steps/os/b022_write_udev_rules.go`)

#### 修改内容
- **华为磁盘规则**：基于 WWID 生成规则，使用 SYMLINK 模式
- **非华为磁盘规则**：基于 DM_NAME 生成规则，统一改为 SYMLINK 模式
- **/dev/yfs 目录管理**：自动创建和配置目录

#### /dev/yfs 目录管理
在配置 UDEV 规则之前，自动执行以下操作：

1. **检查目录是否存在**
   ```bash
   test -d /dev/yfs
   ```

2. **创建目录（如果不存在）**
   ```bash
   mkdir -p /dev/yfs
   ```

3. **设置属主和属组**
   ```bash
   chown yashan:YASDBA /dev/yfs
   ```
   - 属主：从参数 `yac_udev_owner` 获取（默认：yashan）
   - 属组：从参数 `yac_udev_group` 获取（默认：YASDBA）

4. **设置权限**
   ```bash
   chmod 0755 /dev/yfs
   ```

#### 华为磁盘 UDEV 规则格式
```
SUBSYSTEM=="block", ATTR{wwid}=="<WWID>", SYMLINK+="yfs/<diskname>", OWNER="yashan", GROUP="YASDBA", MODE="0666"
```

**示例**：
```
SUBSYSTEM=="block", ATTR{wwid}=="710015ae8a0de929bc4ca0950000000c", SYMLINK+="yfs/ultrapathaa", OWNER="yashan", GROUP="YASDBA", MODE="0666"
```

#### 非华为多路径磁盘 UDEV 规则格式（统一为SYMLINK模式）
```
SUBSYSTEM=="block", ENV{DM_NAME}=="<alias>", SYMLINK+="yfs/<alias>", OWNER="yashan", GROUP="YASDBA", MODE="0666"
```

**示例**：
```
SUBSYSTEM=="block", ENV{DM_NAME}=="data1", SYMLINK+="yfs/data1", OWNER="yashan", GROUP="YASDBA", MODE="0666"
```

#### 默认规则（通配符匹配）
```
SUBSYSTEM=="block", ENV{DM_NAME}=~"^(data|sys|arch)", SYMLINK+="yfs/%E{DM_NAME}", OWNER="yashan", GROUP="YASDBA", MODE="0666"
```

#### 处理流程
1. 遍历所有磁盘组（systemdg、datadg、archdg）
2. 对于每个磁盘：
   - 如果是华为磁盘：获取 WWID，生成基于 WWID 的 SYMLINK 规则
   - 如果是其他多路径磁盘：生成基于 DM_NAME 的 SYMLINK 规则
3. 如果没有生成任何规则：使用默认的通配符 SYMLINK 规则作为后备方案
4. 所有规则都使用变量引用：owner、group、mode

### 4. WWID 收集 (`internal/steps/os/b018_collect_wwid.go`)

#### 修改内容
- **跳过华为磁盘**：在 `collectDisksWWID` 函数中添加华为磁盘检测
- **检测逻辑**：
  ```go
  if commonos.IsHuaweiMultipathDisk(disk) {
      ctx.Logger.Info("  %s is Huawei multipath disk, skipping WWID collection", disk)
      continue
  }
  ```
- **原因**：华为磁盘的 WWID 在 UDEV 规则配置时单独获取，不需要在此步骤收集

## 使用示例

### 场景1：混合磁盘环境（华为 + 标准多路径）

```bash
yinstall db \
  --targets 10.10.10.135,10.10.10.136 \
  --yac-systemdg "sysdg:/dev/ultrapathaa,/dev/ultrapathab" \
  --yac-datadg "datadg:/dev/mapper/data1,/dev/mapper/data2" \
  --skip-os
```

**处理流程**：
- `/dev/ultrapathaa` 和 `/dev/ultrapathab`：跳过多路径配置，生成基于 WWID 的 UDEV 规则
- `/dev/mapper/data1` 和 `/dev/mapper/data2`：生成基于 DM_NAME 的 UDEV 规则

### 场景2：纯华为磁盘环境

```bash
yinstall db \
  --targets 10.10.10.135,10.10.10.136 \
  --yac-systemdg "sysdg:/dev/ultrapathaa" \
  --yac-datadg "datadg:/dev/ultrapathab" \
  --skip-os
```

**处理流程**：
- 所有磁盘都是华为磁盘
- 跳过多路径配置
- 为每个磁盘生成基于 WWID 的 UDEV 规则

## 获取华为磁盘 WWID

如果需要手动获取华为磁盘的 WWID，可以使用以下命令：

```bash
# 方法1：使用 udevadm info -a
udevadm info -a --name /dev/ultrapathaa | grep ATTR{wwid}

# 方法2：使用 udevadm info --query=property
udevadm info --query=property --name /dev/ultrapathaa | grep ID_WWN

# 方法3：使用 udevadm info --query=property
udevadm info --query=property --name /dev/ultrapathaa | grep ID_SERIAL
```

**输出示例**：
```
ATTR{wwid}=="710015ae8a0de929bc4ca0950000000c"
ID_WWN=0x710015ae8a0de929
ID_SERIAL=HUAWEI_STORAGE_710015ae8a0de929bc4ca0950000000c
```

## 日志输出示例

### 多路径配置步骤（B-019）
```
Processing systemdg 'sysdg':
  /dev/ultrapathaa is Huawei multipath disk, skipping multipath configuration
  /dev/mapper/data1 -> data1 (wwid: 360000000000000000000000000000001)
Created /etc/multipath.conf with 1 multipath entries
```

### UDEV 规则配置步骤（B-022）
```
Checking and creating /dev/yfs directory...
  Directory /dev/yfs does not exist, creating it
  Directory /dev/yfs created successfully
Setting ownership of /dev/yfs to yashan:YASDBA...
  Set owner and group for /dev/yfs to yashan:YASDBA
Setting permissions of /dev/yfs to 0755...
  Set permissions for /dev/yfs to 0755
Generated WWID-based rule for Huawei disk /dev/ultrapathaa (wwid: 710015ae8a0de929bc4ca0950000000c)
Generated SYMLINK-based rule for multipath disk /dev/mapper/data1 (alias: data1)
Using default SYMLINK-based rule for all multipath disks
```

### WWID 收集步骤（B-018）
```
Collecting disk WWID information...
  /dev/ultrapathaa is Huawei multipath disk, skipping WWID collection
  /dev/mapper/data1: 360000000000000000000000000000001
Successfully collected WWID for 1 disks
```

## 兼容性

- ✓ 完全兼容现有的标准多路径磁盘配置
- ✓ 支持混合环境（华为 + 标准多路径）
- ✓ 向后兼容：不使用华为磁盘时行为不变
- ✓ 所有现有命令行参数继续有效

## 故障排查

### 问题1：无法获取华为磁盘 WWID

**症状**：日志显示 "Failed to get WWID for Huawei disk"

**解决方案**：
1. 确认磁盘路径正确（应以 `/dev/ultrapath` 开头，如 `/dev/ultrapathaa`）
2. 运行 `udevadm info -a --name /dev/ultrapathaa` 检查磁盘属性
3. 确认磁盘在系统中可见：`ls -la /dev/ultrapath*`

### 问题2：UDEV 规则未生效

**症状**：磁盘权限仍为默认值

**解决方案**：
1. 检查 UDEV 规则文件：`cat /etc/udev/rules.d/99-yashandb-permissions.rules`
2. 重新加载 UDEV 规则：`udevadm control --reload-rules && udevadm trigger`
3. 验证规则生效：`ls -la /dev/yfs/`

### 问题3：多路径配置被跳过

**症状**：日志显示 "is Huawei multipath disk, skipping multipath configuration"

**说明**：这是正常行为，华为磁盘不需要多路径配置。如果需要强制配置，使用 `--force B-019` 参数。
