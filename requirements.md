# 崖山安装自动化工具（Go CLI）需求说明（设计阶段）

> 本文档目标：将 `installer.md` 的最佳实践落地为“可分步、可选择、可审计”的自动化工具，支持一键安装/一键创建备库/一键安装 YCM/一键安装 YMP，但必须拆分为独立命令，并支持自定义执行每一步。  
> **注意：本文档仅描述需求，不包含任何代码与脚本实现。**

---

## 术语与范围

### 术语

- **目标主机**：被安装/配置的服务器（数据库主机、备库主机、YCM 主机、YMP 主机等）。
- **控制端**：运行本工具的机器（任何平台：macOS/Linux/Windows）。
- **本机执行**：控制端与目标主机为同一台机器时，在目标主机本地执行，不做 SSH 认证。
- **远程执行**：控制端通过 SSH 连接目标主机执行命令与校验。
- **步骤（Step）**：可独立执行、可跳过、可重试的最小操作单元，包含前置条件/执行动作/校验/输出。
- **任务（Task）**：步骤的有序集合（例如“安装数据库单机”）。
- **计划（Plan）**：工具根据输入生成的任务/步骤执行计划，可预览、可导出。
- **状态（State）**：工具对每一步执行结果与产物的记录，用于断点续跑/审计。

### 目标能力（必须实现）

- **一键安装崖山数据库**（按选定部署形态：单机 / YAC）
- **一键创建备库**（在已有主库/集群基础上扩容新增备库节点与实例）
- **一键安装 YCM**
- **一键安装 YMP**
- 以上能力均需支持：
  - **拆分为独立命令**
  - **细粒度步骤管理**
  - **用户可自定义执行每一步**（选择/跳过/只运行某些步骤）
  - **跨平台控制端**（通过 SSH 控制目标主机）

### 非目标（暂不纳入首版）

- 采购/硬件上架/BIOS 手工配置自动化（允许做检查与提示，但不强制自动化）
- 网络交换机/VLAN/存储阵列侧配置自动化（允许做校验与提示）
- 完整的迁移任务编排与评估（YMP 的“安装与可用性验证”在范围内，迁移任务本身不在首版范围）

---

## 总体设计原则

### 可控性与可审计

- 每一步必须具备：
  - **唯一 StepID**
  - **输入参数声明**
  - **前置条件检查（Pre-check）**
  - **执行动作（Action）**
  - **结果校验（Post-check）**
  - **输出产物（Artifacts）**（例如生成的配置文件路径、服务状态、端口占用检查结果）
- 所有执行过程必须可追溯：
  - **控制端本地记录**：结构化日志 + 人类可读日志
  - **可选的目标端记录**：将关键日志/产物同步到目标端指定目录

### 幂等与安全

- 默认模式下，步骤应尽量设计为**幂等**：
  - 已存在资源（用户/目录/配置文件/服务/安装产物）应检测并按策略处理：跳过 / 更新 / 报错退出
- 对破坏性操作必须显式确认（或提供强制参数）：
  - 例如：清理目录、删除实例、覆盖配置文件、卸载组件
- 涉及密码/密钥等敏感信息：
  - **禁止在控制台明文输出**
  - 日志中必须进行脱敏（仅保留必要的定位信息）

### 可扩展与跨平台

- 控制端只负责编排与远程执行，不依赖目标端预装特定运行时（除 SSH、基础系统命令外）。
- 目标端命令集存在平台差异（RHEL7/8、UOS、麒麟等），必须通过“**OS 探测 + 适配层**”选择不同步骤分支或不同动作。

---

## 用户画像与使用场景

### 角色

- **交付工程师**：希望按最佳实践快速、可重复地交付环境；需要一键能力与标准化日志。
- **客户运维**：希望可选择性执行，避免“一键黑盒”；需要明确的校验、错误定位与回滚/清理指引。
- **研发/支持**：需要可复现、可导出的执行计划与状态数据，便于排障。

### 典型场景

- 新装单机数据库（含 OS 基线、依赖、安装、建库、环境变量）
- 新装 YAC 集群数据库（含共享盘/多路径/SCAN/VIP 相关校验）
- 已有主库环境，新增备库（节点扩容 + 实例扩容）
- 安装 YCM（sqlite 或 YashanDB 作为后端）
- 安装 YMP（含 JDK/Oracle instantclient/sqlplus 等依赖与可用性验证）

---

## CLI 产品形态与命令体系（需求）

### 可执行程序

- 建议命名：`yinstall`（可调整）

### 顶层命令（必须）

- **`os prepare`**：执行操作系统基线准备（共享模块，可被 db install/YAC/新增备库复用，也可单独运行）
- **`db install`**：安装数据库软件并可选创建数据库（单机/YAC）
- **`db deploy`**：仅创建数据库/集群（依赖软件已安装且所需产物已就绪，例如生成的 hosts/cluster 文件或等价输入）
- **`db standby add`**：在现有环境新增备库（扩容）
- **`ycm install`**：安装并初始化 YCM
- **`ymp install`**：安装并初始化 YMP
- **`plan`**：根据参数生成可执行计划（不落地、不执行）
- **`run`**：执行计划/执行指定任务（支持按 Step 选择）
- **`status`**：查询最近一次或指定 RunID 的执行状态
- **`artifact`**：导出产物与日志（打包/汇总）

### 通用运行模式（必须）

- **预检模式（precheck）**：只做环境与依赖校验，不执行变更
- **试运行（dry-run）**：生成计划并展示将执行哪些步骤与影响范围
- **执行（apply）**：真正执行
- **断点续跑（resume）**：从失败步骤继续

### 步骤选择能力（必须）

- 支持：
  - **按 StepID 列表执行**
  - **按标签（tag）执行**（如 `os`, `db`, `ycm`, `ymp`, `network`, `firewall`）
  - **跳过指定 StepID/标签**
  - **只执行前置检查**
- 需要提供“列出步骤”的能力：
  - 展示：StepID、名称、描述、前置条件、影响范围、可回滚性、默认是否启用

---

## 远程执行与连接管理（SSH）

### 连接策略（必须）

- 支持目标主机清单（单台/多台）：
  - 数据库单机：1 台
  - 主备：2 台（主库、备库）
  - YAC：2 台及以上
  - YCM/YMP：可与数据库同机或独立主机
- SSH 认证方式（必须）：
  - **密码登录**
  - **密钥免密登录**
- **本机执行规则（必须）**：
  - 若目标主机与控制端判定为同机，则不进行 SSH 认证，直接本机执行
  - 判定方式需可配置（例如显式指定 `--local`，或通过 host=127.0.0.1/localhost 等规则）

### 并发与顺序（必须）

- 支持并发执行（例如对多节点重复的 OS 基线步骤）：
  - 并发度由工具内置策略决定（无需对外暴露参数）
  - 保证有依赖关系的步骤按顺序执行（例如先装软件后建库）

### 权限模型（必须）

- 目标端可能需要 root 权限执行系统配置：
  - 工具必须支持以普通用户 SSH 登录，并通过 `sudo` 执行需要特权的步骤
  - 必须具备“sudo 可用性检查”步骤
- 需明确两类账户：
  - **系统管理员账户**（root 或具 sudo 权限）
  - **产品安装账户**（如 `yashan`、`ymp`）

---

## 参数化与配置（核心要求）

### 总原则（必须）

- `installer.md` 中出现的“用户名、密码、组 ID、端口、目录、软件包路径、ISO 路径、DNS、网卡名、VIP/SCAN”等，**必须全部可参数化**。
- 不允许在工具内部写死：
  - 默认值可以有，但必须可覆盖
  - 必须支持通过命令行参数提供（敏感信息可通过环境变量/交互注入）

### 配置载入（必须）

- **仅支持命令行参数**：本工具不支持配置文件输入（不提供 `--config`）。
- **环境变量/交互输入**：仅用于注入敏感信息（密码/密钥/Token 等），不作为通用配置载体。
- **优先级**：命令行 > 默认值；敏感信息来源：环境变量/交互 > 默认（默认视为未提供）。

### 敏感信息处理（必须）

- 密码/密钥/Token 等必须支持：
  - 通过环境变量或交互输入提供
  - 控制端日志脱敏
  - 产物文件（如 hosts 配置）若包含密文/密钥，应明确其保护方式与权限设置要求

### 软件包目录与自动分发（必须）

> 目标：支持“控制端软件目录 + 目标端软件目录”的两级查找与自动上传，避免用户为每个文件填写绝对路径。

- **目录参数（必须）**：
  - **控制端软件目录（local software dir）**：脚本执行端存放安装包的目录（可配置多个，按顺序查找）
  - **目标端软件目录（remote software dir）**：目标主机用于缓存安装包的目录（可配置；不存在时可由步骤创建）
- **查找与分发规则（必须）**：
  - 当某 Step 需要某个安装包（DB/YCM/YMP/deps/ISO/instantclient/JDK 等）时，按以下顺序解析来源：
    1. **先在目标端软件目录查找**（已存在则直接使用，不重复上传）
    2. 若目标端不存在，则在**控制端软件目录查找**
    3. 若控制端找到，则**自动上传**到目标端软件目录，并使用上传后的目标端路径继续后续步骤
    4. 若两端都找不到，则失败，并输出“期望文件名/期望目录列表”
- **文件一致性校验（必须）**：
  - 上传后必须校验：文件大小一致；可选支持 checksum（sha256）校验（由参数控制）
  - 若目标端已存在同名文件：
    - 默认策略：校验通过则复用；校验不通过则失败（除非用户显式允许覆盖）
- **权限与属主（必须）**：
  - 目标端软件目录的属主/权限策略需可配置（默认保持安全权限，避免全员可写）
- **日志要求（必须）**：
  - debug 日志需记录：解析到的来源（remote/local）、上传动作、校验结果、最终使用的目标端路径
  - 禁止在日志中泄露包含敏感信息的路径片段（如凭据文件名）时需脱敏（若存在此类场景）

### 目录结构设计（必须）

> 目标：定义“脚本/工具”的目录结构，包含**源码仓库结构**与**运行时目录结构（控制端/目标端）**，保证可维护、可审计、可复用，并与本文参数默认值一致。

#### 1) 源码仓库目录结构（Go 项目，建议）

- **`docs/`**：文档（`installer.md`、`requirements.md`、使用说明等）
- **`cmd/yinstall/`**：CLI 入口（命令定义与参数解析）
- **`internal/`**：核心实现（不对外暴露）
  - **`internal/ssh/`**：SSH/本机执行通道
  - **`internal/runner/`**：Step 执行器、依赖编排、内置并发/重试策略
  - **`internal/steps/`**：步骤库（os/db/yac/standby/ycm/ymp 分组）
  - **`internal/state/`**：RunID/状态落盘、resume/status
  - **`internal/logging/`**：屏幕简洁日志 + debug 全量日志
  - **`internal/artifacts/`**：计划/报告/关键产物汇总与打包
  - **`internal/redact/`**：统一脱敏规则
  - **`internal/templates/`**：模板与渲染（sysctl/repo/multipath/udev/systemd 等）
- **`templates/`**：可选的模板文件目录（若不放在 `internal/templates/`）
- **`examples/`**：常见场景命令行示例（single/yac/standby/ycm/ymp）
- **`testdata/`**：测试数据与样例输出（不包含敏感信息）

#### 2) 控制端（脚本执行端）运行时目录结构（默认固定）

> 说明：本工具不提供 `--artifact-dir` 等额外参数；运行时目录采用固定结构。\n+> 允许通过 `--log-dir` 改变“日志目录”，其余目录固定在 `~/.yinstall/` 下。

- **根目录（固定）**：`~/.yinstall/`
  - **`logs/`**：日志根目录（默认；若传 `--log-dir` 则使用其值作为日志根目录）
    - **`<run_id>/`**：一次运行一个目录
      - `console.log`：屏幕简洁日志落盘副本（可选）
      - **`debug/`**：debug 全量日志
        - `host-<host>.log`（或按 Step 分段的多个文件；策略由工具内置）
      - `summary.json`：本次运行摘要（成功/失败 Step 列表、关键错误摘要）
  - **`state/`**
    - **`<run_id>/state.json`**：状态文件（用于 resume/status）
  - **`artifacts/`**
    - **`<run_id>/`**：产物目录（计划/报告/打包文件）
      - `plan.json`（或等价的计划输出）
      - `report.md`（可选：人类可读报告）
      - `bundle.zip`（可选：打包日志/状态/关键文件摘要）
  - **`tmp/`**
    - **`<run_id>/`**：控制端临时文件（上传缓存、checksum 中间文件等）

#### 3) 目标端（服务器）目录结构（默认，可通过参数覆盖）

- **软件缓存目录（默认）**：`/data/yashan/soft`（对应 `--remote-software-dir`）\n+  - 建议子目录（可选）：`db/`、`ycm/`、`ymp/`、`deps/`、`iso/`
- **执行临时目录（固定规则）**：`/var/tmp/yinstall/<run_id>/`\n+  - 用途：临时配置副本、探测结果、校验输出等\n+  - 清理策略：由工具内置策略处理（不对外暴露参数）
- **YashanDB 默认目录**（可参数化）：\n+  - `--db-install-path`：`/data/yashan/yasdb_home`\n+  - `--db-data-path`：`/data/yashan/yasdb_data`\n+  - `--db-log-path`：`/data/yashan/log`\n+  - `--db-stage-dir`：`/home/yashan/install`
- **YCM 默认目录**（可参数化）：\n+  - `--ycm-install-dir`：默认 `/opt`（即 `/opt/ycm`）\n+  - `--ycm-deploy-file`：`/opt/ycm/etc/deploy.yml`
- **YMP 默认目录**（可参数化）：\n+  - `--ymp-install-dir`：`/opt/ymp`\n+  - `--ymp-oracle-env-file`：`/home/ymp/.oracle`

#### 4) 目录权限与属主（必须约束）

- **目标端软件缓存目录**：默认 root 可写、其他用户不可写（避免被篡改）；必要时通过 sudo 管理\n+- **产品安装目录**：由对应产品用户（如 `yashan`/`ymp`）拥有并可写\n+- **控制端目录**（`~/.yinstall`）：必须保证仅当前用户可读写（建议 0700），避免泄露敏感信息

### 参数设计（必须：尽可能覆盖 installer.md，且提供默认值）

> 说明：
> - 参数全部来自命令行；本文仅定义“参数名、含义、默认值、何时必填”。
> - 对于“依赖安装包文件”的参数，默认值通常是**默认文件名**或**空值表示不启用该能力**；真正执行时仍需通过“软件包目录与自动分发”机制定位到实际文件。
> - 所有敏感参数（密码/密钥）默认值视为“未提供”，需要通过环境变量/交互/密文文件注入。

#### 参数命名与分组规范

- **命名风格**：
  - CLI：`--kebab-case`
- **说明**：不提供配置文件参数映射规则（本工具仅支持 CLI）。
- **布尔开关**：默认 `false`（除非最佳实践明确建议默认启用）
- **列表参数**：默认空列表 `[]`
- **路径参数**：默认使用最佳实践推荐路径（可覆盖）
- **端口参数**：默认采用最佳实践默认端口（可覆盖）

#### 1) 全局参数（所有命令通用）

- **`--run-id`**：指定 RunID（用于 resume/status）；默认：自动生成
- **`--dry-run`**：只生成计划不执行；默认：false
- **`--precheck`**：只做检查不变更；默认：false
- **`--resume`**：断点续跑；默认：false
- **`--include-steps`**：仅执行指定 StepID 列表；默认：[]
- **`--exclude-steps`**：跳过指定 StepID 列表；默认：[]
- **`--include-tags`**：仅执行指定标签（如 `os`,`db`,`yac`,`ycm`,`ymp`）；默认：[]
- **`--exclude-tags`**：跳过指定标签；默认：[]
#### 2) 目标主机与 SSH 连接参数（所有远程相关命令通用）

- **`--targets`**：目标主机列表（IP/hostname）；默认：[]
  - 规则：单机/组件可为 1 台；YAC ≥ 2；主备通常 2
- **`--ssh-port`**：SSH 端口；默认：22
- **`--ssh-user`**：SSH 登录用户；默认：`root`
- **`--ssh-auth`**：认证方式（`password|key|local`）；默认：`key`
- **`--ssh-password`**：SSH 密码；默认：未提供（仅当 `ssh-auth=password` 时必填）
- **`--ssh-key-path`**：私钥路径；默认：`~/.ssh/id_rsa`
- **`--ssh-key-passphrase`**：私钥口令；默认：未提供（可选）
- **`--ssh-known-hosts`**：known_hosts 策略（strict/accept-new/ignore）；默认：`accept-new`
- **`--local`**：强制本机执行（不做认证）；默认：false
- **`--sudo`**：执行需要提权的步骤时使用 sudo；默认：true

#### 3) 日志与产物参数（所有命令通用）

- **`--log-dir`**：控制端日志目录；默认：`~/.yinstall/logs`

#### 4) 软件包目录与文件定位参数（所有安装类命令通用）

- **`--local-software-dirs`**：控制端软件目录列表；默认：[`./software`, `./pkg`]
- **`--remote-software-dir`**：目标端软件目录；默认：`/data/yashan/soft`
- **`--software-owner`**：目标端软件目录属主；默认：`root`
- **`--software-group`**：目标端软件目录属组；默认：`root`
- **`--software-mode`**：目标端软件目录权限；默认：`0750`
- **`--software-verify-sha256`**：上传后是否做 sha256 校验；默认：false
- **`--software-allow-overwrite`**：目标端同名文件校验不一致时是否允许覆盖；默认：false

#### 5) OS 共享模块（`os prepare`）参数

- **账户/组（对应 B-001~B-004）**
  - **`--os-user`**：产品安装用户；默认：`yashan`
  - **`--os-user-uid`**：用户 UID；默认：701
  - **`--os-group`**：主组名；默认：`yashan`
  - **`--os-group-gid`**：主组 GID；默认：701
  - **`--os-dba-group`**：附加 DBA 组；默认：`YASDBA`
  - **`--os-dba-group-gid`**：DBA 组 GID；默认：702
  - **`--os-user-shell`**：用户 shell；默认：`/bin/bash`
  - **`--os-user-password`**：用户密码；默认：未提供（可选）
  - **`--os-sudoers-enable`**：是否写 sudoers NOPASSWD；默认：true
- **时区/时间同步（对应 B-005、B-013）**
  - **`--os-timezone`**：时区；默认：`Asia/Shanghai`
  - **`--os-ntp-server`**：NTP server；默认：`ntp.aliyun.com`
- **sysctl/limits/启动参数（对应 B-006~B-009）**
  - **`--os-sysctl-file`**：sysctl 写入文件；默认：`/etc/sysctl.d/yashandb.conf`
  - **`--os-sysctl-params`**：sysctl 参数集合（键值对）；默认：采用最佳实践模板（可覆盖任一键）
  - **`--os-limits-file`**：limits 文件；默认：`/etc/security/limits.conf`
  - **`--os-limits-profile`**：limits 模板名；默认：`yashandb_default`
  - **`--os-kernel-args-enable`**：是否写启动参数；默认：false
  - **`--os-kernel-args`**：启动参数字符串；默认：`transparent_hugepage=never elevator=deadline LANG=en_US.UTF-8`
- **YUM 本地源/ISO（对应 B-010~B-012）**
  - **`--os-yum-mode`**：包源模式（online/local-iso/none）；默认：`none`
  - **`--os-iso-device`**：ISO 设备（如 `/dev/cdrom`）；默认：`/dev/cdrom`
  - **`--os-iso-mountpoint`**：挂载点；默认：`/media`
  - **`--os-yum-repo-file`**：repo 文件路径；默认：`/etc/yum.repos.d/local.repo`
  - **`--os-deps-db-packages`**：DB 依赖包列表；默认：`libzstd,zlib,lz4,openssl,openssl-devel`
  - **`--os-deps-tools-packages`**：常用工具包列表；默认：空（不安装）
- **防火墙（对应 B-014/B-015）**
  - **`--os-firewall-mode`**：防火墙模式（keep/disable/open-ports）；默认：`keep`
  - **`--os-firewall-ports`**：开放端口列表；默认：[]（当 mode=open-ports 才生效）
- **YAC 多路径 + udev（对应 B-017~B-024，仅 YAC 需要）**
  - **`--yac-multipath-enable`**：是否启用多路径步骤；默认：false
  - **`--yac-multipath-packages`**：多路径包列表；默认：`device-mapper-multipath`
  - **`--yac-multipath-conf`**：multipath.conf 路径；默认：`/etc/multipath.conf`
  - **`--yac-multipath-alias-map`**：WWID→alias 映射；默认：[]（未提供时仅可做校验或需要自动采集）
  - **`--yac-multipath-auto-wwid`**：是否自动采集 WWID；默认：false
  - **`--yac-udev-rules-file`**：udev rules 文件；默认：`/etc/udev/rules.d/99-yashandb-permissions.rules`
  - **`--yac-udev-owner`**：磁盘属主；默认：`yashan`
  - **`--yac-udev-group`**：磁盘属组；默认：`YASDBA`
  - **`--yac-udev-mode`**：磁盘权限；默认：`0666`

#### 6) DB 安装（单机/YAC）参数（`db install`/`db deploy`）

- **主机（必须：用 IP 列表驱动部署形态）**
  - **`--db-ips`**：数据库节点 IP 列表；默认：[]
    - **规则（必须）**：
      - `len(db_ips)=1`：判定为**单机**
      - `len(db_ips)>=2`：判定为 **YAC**
    - **与 `--targets` 的关系（必须明确）**：
      - 若提供 `--db-ips`：DB 相关步骤以 `db_ips` 为准（`--targets` 仍可用于非 DB 任务，例如独立执行 `os prepare`）
      - 若未提供 `--db-ips` 但提供 `--targets`：可将 `targets` 视为 `db_ips`（兼容模式），但必须在 debug 日志中记录该推导
      - 若两者都提供且不一致：默认失败（除非提供显式兼容开关）
- **通用**
  - **`--db-cluster-name`**：cluster 名；默认：`yashandb`
  - **`--db-begin-port`**：起始端口；默认：1688
  - **`--db-memory-limit-percent`**：内存百分比；默认：70
  - **`--db-character-set`**：字符集；默认：`utf8`
  - **`--db-use-native-type`**：原生类型；默认：true
  - **`--db-admin-password`**：管理员口令；默认：未提供（建库时必填）
- **路径（默认按最佳实践）**
  - **`--db-install-path`**：软件安装目录；默认：`/data/yashan/yasdb_home`
  - **`--db-data-path`**：数据目录；默认：`/data/yashan/yasdb_data`
  - **`--db-log-path`**：日志目录；默认：`/data/yashan/log`
  - **`--db-stage-dir`**：解压/执行目录；默认：`/home/yashan/install`
- **安装包定位**
  - **`--db-package`**：DB 安装包文件名或路径；默认：空（必须能通过软件目录机制解析到）
  - **`--db-deps-package`**：SSL deps 包文件名或路径；默认：空（不使用）
- **模式**
  - **`--db-mode`**：single/yac；默认：由 `db_ips` 自动推导
    - 说明：该参数仅作为“强制覆盖推导结果”的高级用法（一般不建议使用）
  - **`--db-nodes`**：节点数；默认：由 `len(db_ips)` 推导

#### 7) DB YAC 专属参数（仅 `--db-mode=yac`）

- **网络（对应 D-006 的生成配置）**
  - **`--yac-inter-cidr`**：集群私网 CIDR；默认：空（YAC 必填）
  - **`--yac-public-network`**：业务网定义（CIDR/网卡名）；默认：空（YAC 必填）
  - **`--yac-access-mode`**：vip/scan；默认：`vip`
  - **`--yac-vips`**：VIP 列表；默认：[]（access-mode=vip 时必填）
  - **`--yac-scanname`**：SCAN 名称；默认：空（access-mode=scan 时必填）
- **共享盘（对应 D-006）**
  - **`--yac-disk-found-path`**：磁盘发现路径；默认：`/dev/mapper`
  - **`--yac-system-disks`**：system 磁盘列表；默认：[]（YAC 必填）
  - **`--yac-data-disks`**：data 磁盘列表；默认：[]（YAC 必填）
  - **`--yac-arch-disks`**：arch 磁盘列表；默认：[]（可选）
- **YFS 参数（对应 D-007，可选）**
  - **`--yac-yfs-tune-enable`**：是否调参；默认：false
  - **`--yac-yfs-au-size`**：默认：`32M`
  - **`--yac-redo-file-size`**：默认：`1G`
  - **`--yac-redo-file-num`**：默认：6
  - **`--yac-shm-pool-size`**：默认：`2G`
  - **`--yac-max-instances`**：默认：64

#### 8) 新增备库（`yinstall standby`）参数

> 命令简化为 `yinstall standby`，执行新增备库（扩容）操作。

- **操作系统配置控制**
  - **`--with-os`**：是否配置备库节点的操作系统；默认：`true`
    - `true`：配置操作系统（在备库节点执行 B-xxx 步骤，与主库安装时的 OS 配置一致）
    - `false`：跳过操作系统配置（仅执行 E-xxx 备库扩容步骤）
    - **注意**：即使 `--with-os=false`，仍会执行 B-000（连通性检查）确保备库节点可达
- **主库信息（必须）**
  - **`--primary-ip`**：主库 IP 地址；默认：空（必填）
    - 用于连接主库执行扩容命令（`yasboot config node gen`、`yasboot host add`、`yasboot node add`）
  - **`--primary-ssh-user`**：主库 SSH 用户名；默认：取 `--ssh-user` 的值
  - **`--primary-ssh-password`**：主库 SSH 密码；默认：取 `--ssh-password` 的值
  - **`--primary-ssh-key`**：主库 SSH 私钥路径；默认：取 `--ssh-key` 的值
- **备库目标节点（必须）**
  - **`--targets`**：备库目标节点 IP 列表（复用全局参数）；默认：[]（必填）
    - 规则：`len(targets)>=1`，支持批量新增多个备库节点
- **数据库集群信息（必须）**
  - **`--db-cluster-name`**：数据库集群名称；默认：`yashandb`（必须与主库集群名一致）
  - **`--db-admin-password`**：数据库 SYS 管理员密码；默认：空（必填，用于备库实例创建）
- **操作系统用户信息（用于备库节点）**
  - **`--os-user`**：备库节点产品安装用户；默认：`yashan`
  - **`--os-user-password`**：备库节点用户密码；默认：空（用于 `yasboot config node gen` 命令）
  - **`--os-group`**：备库节点主组名；默认：`yashan`
- **路径（必须与主库保持一致）**
  - **`--db-install-path`**：软件安装目录；默认：`/data/yashan/yasdb_home`
  - **`--db-data-path`**：数据目录；默认：`/data/yashan/yasdb_data`
  - **`--db-log-path`**：日志目录；默认：`/data/yashan/log`
  - **`--db-stage-dir`**：主库解压/执行目录（扩容命令在此执行）；默认：`/home/yashan/install`
- **安装包定位（可选）**
  - **`--db-deps-package`**：SSL deps 包文件名或路径；默认：空（不使用）
- **扩容控制**
  - **`--standby-node-count`**：新增节点数量；默认：由 `len(targets)` 自动推导
  - **`--standby-cleanup-on-failure`**：扩容失败时是否自动清理；默认：`false`（危险操作，需配合 `--force` 使用）
- **复用全局参数**：`--ssh-user`、`--ssh-auth`、`--ssh-password`、`--ssh-key`、`--ssh-port` 等（用于备库节点连接）

#### 9) YCM 安装（`ycm install`）参数

- **路径与包**
  - **`--ycm-install-dir`**：安装目录；默认：`/opt`
  - **`--ycm-package`**：YCM 安装包文件名或路径；默认：空（需可解析）
  - **`--ycm-deploy-file`**：deploy 配置文件路径；默认：`/opt/ycm/etc/deploy.yml`
- **端口（默认取最佳实践）**
  - **`--ycm-port`**：默认：9060
  - **`--ycm-prometheus-port`**：默认：9061
  - **`--ycm-loki-http-port`**：默认：9062
  - **`--ycm-loki-grpc-port`**：默认：9063
  - **`--ycm-yasdb-exporter-port`**：默认：9064
- **后端 DB**
  - **`--ycm-db-driver`**：sqlite3/yashandb；默认：`sqlite3`
  - **`--ycm-db-url`**：后端 DB 地址；默认：空（driver=yashandb 时必填）
  - **`--ycm-db-lib-path`**：客户端 libPath；默认：空（可选）
  - **`--ycm-db-admin-user`**：默认：`yasman`
  - **`--ycm-db-admin-password`**：默认：未提供（driver=yashandb 时必填）

#### 10) YMP 安装（`ymp install`）参数

- **用户与目录**
  - **`--ymp-user`**：默认：`ymp`
  - **`--ymp-install-dir`**：默认：`/opt/ymp`
- **端口**
  - **`--ymp-port`**：默认：8090
- **JDK**
  - **`--ymp-jdk-enable`**：是否安装 JDK；默认：false（允许只校验）
  - **`--ymp-jdk-version`**：默认：11
  - **`--ymp-jdk-package`**：JDK 包文件名或路径；默认：空（启用安装时必填）
- **软件包**
  - **`--ymp-package`**：YMP zip 文件名或路径；默认：空（需可解析）
  - **`--ymp-instantclient-basic`**：instantclient basic 包；默认：空（需可解析）
  - **`--ymp-instantclient-sqlplus`**：sqlplus 包；默认：空（可选）
  - **`--ymp-db-package`**：用于安装内置库的 DB 包；默认：空（需可解析）
- **可选：sqlplus 环境文件**
  - **`--ymp-oracle-env-file`**：写入 oracle env 的文件；默认：`/home/ymp/.oracle`

---

## 任务与步骤库设计（基于 installer.md 的拆分）

> 说明：以下是首版必须覆盖的“建议步骤集合”。实现时允许继续细化，但不可粗化到不可控。

### 步骤设计规范（必须：原子化）

- **单一职责**：每个 Step **只能做一个功能**（一个可清晰描述的动作），不得把“写配置 + 生效 + 校验”合并到同一步。
- **统一结构**：每个 Step 至少包含：
  - **Pre-check**：执行前的必要检查（若不满足则失败或跳过）
  - **Action**：唯一的变更动作（或唯一的检查动作）
  - **Post-check**：验证 Action 的结果
  - **Artifacts**：日志/文件/关键输出（路径与摘要）
- **幂等策略**：每个 Step 必须声明对“已存在状态”的处理策略：`skip / update / fail`（可由参数控制）。

### 公共函数/公共操作库（必须：抽取重复操作）

> 以下为需求级“公共能力”，用于把跨步骤重复的操作抽到统一实现中（不在本文给出任何代码）。

- **CF-001 目标主机解析与执行通道选择**
  - **能力**：识别本机/远程；建立 SSH（密码/密钥）或本机执行通道
  - **复用**：所有 Step
- **CF-002 远程命令执行器**
  - **能力**：执行命令（可选 sudo）、超时、重试、捕获 stdout/stderr、脱敏输出
  - **复用**：所有 Step 的 Action/Post-check
- **CF-003 文件分发与采集**
  - **能力**：
    - 上传/下载文件；设置权限与属主；目录不存在则创建（可选）
    - **软件包解析与自动上传**：按“先远端后本地”的规则定位安装包，必要时自动上传到目标端软件目录（见“软件包目录与自动分发”）
  - **复用**：涉及配置文件/软件包/ISO 的步骤
- **CF-004 OS/架构探测**
  - **能力**：识别 OS 发行版/版本、CPU 架构、包管理器类型（yum/dnf 等）
  - **复用**：OS 基线、依赖安装、兼容性校验
- **CF-005 幂等资源管理**
  - **能力**：Ensure/Check 类操作统一实现（如用户/组/目录/行追加/服务 enable）
  - **复用**：创建用户/目录/配置写入/服务管理
- **CF-006 文本文件安全编辑**
  - **能力**：备份文件（带时间戳）；按“插入/替换/追加/幂等行存在”方式修改；权限回收
  - **复用**：`/etc/sysctl.d/*`、`/etc/security/limits.conf`、`/etc/chrony.conf`、`/etc/sudoers`、`.bashrc`
- **CF-007 端口与进程校验**
  - **能力**：端口占用检查、监听校验、进程存在性校验、服务状态校验
  - **复用**：DB/YCM/YMP 可用性验证
- **CF-008 产物与状态登记**
  - **能力**：将每步关键输出登记到 State（RunID、StepID、文件路径、摘要、校验结果）
  - **复用**：所有 Step

### A. 通用步骤（跨任务复用）

- **A-001 OS/架构探测**
  - **Pre-check**：目标主机可执行基础命令（如 `uname`、读取发行版信息）
  - **Action**：采集 OS/版本/内核/架构/包管理器信息（复用 CF-004）
  - **Post-check**：输出字段完整（OS、version、arch、pkgMgr）
  - **Artifacts**：`os_facts.json`（或等价结构化产物）
- **A-002 SSH/本机连通性检查**
  - **Pre-check**：目标主机清单已提供
  - **Action**：对每台主机执行“可连接性检查”（复用 CF-001/CF-002）
  - **Post-check**：返回延迟、认证方式、执行身份
  - **Artifacts**：连接诊断报告（脱敏）
- **A-003 主机名解析检查（DNS/hosts）**
  - **Pre-check**：提供需要解析的域名/主机名（如 SCAN name）
  - **Action**：执行解析检查（如 `nslookup`/`getent hosts`）
  - **Post-check**：解析结果满足预期（如返回多 IP）
  - **Artifacts**：解析结果记录
- **A-004 端口“占用检查（FREE/USED）”**
  - **Pre-check**：端口列表已明确（来自参数/配置）
  - **Action**：检查端口是否被占用（复用 CF-007）
  - **Post-check**：按策略处理：占用则 fail 或提示用户改端口
  - **Artifacts**：端口检查表
- **A-005 目录存在性与空间检查**
  - **Pre-check**：目录列表与最低空间阈值已提供
  - **Action**：检查目录存在/权限/剩余空间（不创建）
  - **Post-check**：满足要求或失败
  - **Artifacts**：目录检查表
- **A-006 sudo 可用性检查**
  - **Pre-check**：登录用户与 sudo 策略已确定
  - **Action**：验证是否可无交互 sudo（复用 CF-002）
  - **Post-check**：可用则通过；不可用则失败或提示执行“可选的 sudoers 配置步骤”
  - **Artifacts**：sudo 检查结果

### B. OS 基线准备（可选但建议标准化）

> **共享模块定位：** OS 基线在“单机安装 / YAC / 新增备库 / YCM / YMP”等场景都会复用，因此必须作为**可独立执行的共享模块**存在（`os prepare`），并支持被其它任务“引用/依赖”，而不是在每个任务里重复定义。
>
> **复用方式要求：**
> - 其它任务（如 `db install`、`db standby add`、`db install --mode=yac`）只能**引用**本模块的 StepID/Tag（如 `tag=os`），不得复制同样的 OS 步骤定义。
> - 本模块的步骤默认可选，允许按项目策略设置“生产强制/测试可选”。
> - 本模块应支持“只做检查（precheck）”与“只执行其中一部分 Step”。

- **B-001 创建产品组（yashan/YASDBA）**
  - **Pre-check**：gid 参数已提供
  - **Action**：创建组（或检测已存在，按策略处理）（复用 CF-005）
  - **Post-check**：组存在且 gid 符合预期
  - **Artifacts**：组信息记录
- **B-002 创建产品用户（如 yashan）**
  - **Pre-check**：uid/主组/附加组/home/shell 参数已提供
  - **Action**：创建用户（或检测已存在，按策略处理）（复用 CF-005）
  - **Post-check**：用户存在且 uid/gid/组关系符合预期
  - **Artifacts**：用户信息记录
- **B-003 设置产品用户密码（可选）**
  - **Pre-check**：密码来源已提供（环境变量/交互/密文文件），且策略允许设置
  - **Action**：设置密码（复用 CF-002；严格脱敏）
  - **Post-check**：可选校验（例如仅校验命令返回码，不回显密码）
  - **Artifacts**：执行记录（脱敏）
- **B-004 配置 sudoers（可选/高风险）**
  - **Pre-check**：明确要授予 sudo 的用户与规则；确认风险开关已开启
  - **Action**：以“安全编辑”方式修改 sudoers，并按最佳实践回收权限（复用 CF-006）
  - **Post-check**：重新执行 A-006 sudo 可用性检查
  - **Artifacts**：sudoers 备份路径与变更摘要
- **B-005 设置时区**
  - **Pre-check**：目标时区参数已提供
  - **Action**：设置时区
  - **Post-check**：读取时区确认已生效
  - **Artifacts**：时区确认输出
- **B-006 写入 sysctl 配置文件**
  - **Pre-check**：sysctl 参数集合已提供；写入路径已确定（如 `/etc/sysctl.d/yashandb.conf`）
  - **Action**：写入/更新配置文件（复用 CF-006）
  - **Post-check**：文件存在且包含预期键
  - **Artifacts**：备份文件与差异摘要
- **B-007 使 sysctl 生效**
  - **Pre-check**：sysctl 配置文件已写入
  - **Action**：执行 sysctl 生效命令
  - **Post-check**：读取关键参数确认生效
  - **Artifacts**：sysctl 输出摘要
- **B-008 写入 limits 配置**
  - **Pre-check**：limits 项（nofile/nproc/memlock 等）已提供
  - **Action**：写入/更新 limits 文件（复用 CF-006）
  - **Post-check**：文件包含预期内容
  - **Artifacts**：备份文件与差异摘要
- **B-009 写入内核启动参数（grubby）（可选）**
  - **Pre-check**：确认目标 OS 支持该方式；参数集已提供
  - **Action**：写入启动参数
  - **Post-check**：读取配置确认已写入；标记“需要重启”
  - **Artifacts**：变更摘要与“需要重启”标记
- **B-010 挂载 ISO（可选：本地源）**
  - **Pre-check**：ISO 设备/路径与挂载点已提供
  - **Action**：挂载
  - **Post-check**：挂载点可读
  - **Artifacts**：`mount` 输出摘要
- **B-011 写入 YUM/DNF 本地 repo 文件（可选）**
  - **Pre-check**：OS 分支明确（RHEL7 vs RHEL8+）；repo 模板参数已提供
  - **Action**：写入 repo 文件（复用 CF-006）
  - **Post-check**：repo 文件存在且基础字段正确
  - **Artifacts**：repo 文件路径与摘要
- **B-012 安装依赖包（DB 通用依赖）**
  - **Pre-check**：包列表已提供；包管理器已探测（A-001）
  - **Action**：安装依赖（区分在线源/本地源分支）
  - **Post-check**：关键包可查询到已安装
  - **Artifacts**：安装输出摘要与失败包清单（如有）
- **B-013 写入 chrony 配置（可选）**
  - **Pre-check**：NTP server 地址已提供
  - **Action**：备份并写入 chrony 配置（复用 CF-006）
  - **Post-check**：重启/启用服务并执行 tracking 校验
  - **Artifacts**：chrony tracking 输出
- **B-014 防火墙：关闭（可选/高风险）**
  - **Pre-check**：风险开关已开启
  - **Action**：stop+disable 防火墙服务
  - **Post-check**：服务状态为 inactive/disabled
  - **Artifacts**：服务状态输出
- **B-015 防火墙：开放端口（可选）**
  - **Pre-check**：端口清单已提供；防火墙开启且可管理
  - **Action**：添加端口规则并 reload
  - **Post-check**：查询端口规则确认存在
  - **Artifacts**：端口规则输出
- **B-016（可选）重启提示与重启后校验**
  - **Pre-check**：存在“需要重启”标记
  - **Action**：生成重启提示/或由用户执行重启
  - **Post-check**：重启后重新执行关键校验（如透明大页/内核参数）
  - **Artifacts**：重启后校验报告

#### B.YAC 共享存储准备（多路径 + udev）（仅 YAC 场景需要）

> 说明：参考 `installer.md` 的 YAC 最佳实践，**当 YAC 使用共享存储且采用多路径（/dev/mapper/*）方式呈现磁盘时**，必须执行以下步骤。  
> 若客户侧由存储/主机工程师提前完成多路径配置，本模块仍应提供“校验”步骤用于确认一致性与权限正确性。

- **B-017（可选）安装多路径软件包**
  - **Pre-check**：已探测 OS/包管理器（A-001）；用户启用多路径模式
  - **Action**：安装多路径相关软件包（例如 `device-mapper-multipath` 等）
  - **Post-check**：`multipath`/`multipathd` 命令可用
  - **Artifacts**：安装摘要
- **B-018（可选）采集共享盘 WWID 信息（用于生成 multipath.conf）**
  - **Pre-check**：用户启用“自动采集 WWID”；具备读取块设备信息权限
  - **Action**：采集目标盘 WWID 列表（每台节点均采集，复用 CF-002）
  - **Post-check**：WWID 采集结果非空，并能跨节点比对一致性（同一共享盘在各节点应返回相同 WWID）
  - **Artifacts**：每节点 WWID 清单
- **B-019（可选）写入 `/etc/multipath.conf`**
  - **Pre-check**：用户提供 WWID→alias 映射（如 `sys1/data01/arch01`）；或来自 B-018 的采集结果；确认风险开关已开启（修改系统配置）
  - **Action**：写入/更新多路径配置文件（复用 CF-006）
  - **Post-check**：文件存在且包含预期 alias/wwid 配置
  - **Artifacts**：备份文件与差异摘要
- **B-020（可选）启用并启动 `multipathd` 服务**
  - **Pre-check**：多路径软件已安装（B-017）
  - **Action**：enable + start 服务
  - **Post-check**：服务状态为 active/enabled
  - **Artifacts**：服务状态输出
- **B-021（必选：YAC 多路径）校验多路径设备是否生效**
  - **Pre-check**：已配置或已存在多路径环境
  - **Action**：执行 `multipath -ll` 并采集结果
  - **Post-check**：能看到预期 alias（如 `sys*|data*|arch*`），且各节点 alias 一致
  - **Artifacts**：`multipath -ll` 输出摘要
- **B-022（可选）写入 udev 权限规则（多路径环境）**
  - **Pre-check**：使用多路径 DM_NAME 命名规则（如 `sys*|data*|arch*`）；目标属主/属组（如 `yashan:YASDBA`）已提供
  - **Action**：写入/更新 udev rules 文件（复用 CF-006），示例规则形态（不限定实现）：
    - `ENV{DM_NAME}=="data*|sys*|arch*"... OWNER/GROUP/MODE`
  - **Post-check**：rules 文件存在且语法满足基本校验
  - **Artifacts**：rules 文件路径与差异摘要
- **B-023（可选）触发 udev 规则生效**
  - **Pre-check**：udev rules 已写入（B-022）
  - **Action**：reload-rules + trigger（复用 CF-002）
  - **Post-check**：命令返回码为 0
  - **Artifacts**：执行输出摘要
- **B-024（必选：YAC 多路径）校验共享盘权限**
  - **Pre-check**：多路径设备存在（B-021），且期望权限/属主已明确
  - **Action**：检查 `/dev/mapper/*`（或 `/dev/dm-*`）的 owner/group/mode
  - **Post-check**：满足预期，否则失败并给出差异
  - **Artifacts**：权限检查表

### C. 数据库安装与建库（单机）

> **依赖关系（必须）：** 单机安装流程可选择依赖 OS 共享模块（见 B 模块/`os prepare`）。实现时应通过“引用 B-xxx StepID 或 tag=os”的方式复用，不得重复定义 OS 步骤。

- **C-001 创建 DB 安装目录（home/install）**
  - **Pre-check**：目标路径参数已提供
  - **Action**：创建目录（复用 CF-005）
  - **Post-check**：目录存在且权限正确
  - **Artifacts**：目录信息记录
- **C-002 创建 DB 数据/日志/软件目录**
  - **Pre-check**：`yasdb_home/yasdb_data/log/soft` 路径参数已提供
  - **Action**：创建目录（不做 chown）
  - **Post-check**：目录存在
  - **Artifacts**：目录列表记录
- **C-003 设置目录属主与权限（chown）**
  - **Pre-check**：安装用户已存在（B-002，来自 OS 共享模块）
  - **Action**：对指定根目录执行属主修正（复用 CF-002/CF-005）
  - **Post-check**：抽样校验属主正确
  - **Artifacts**：变更摘要
- **C-004 解压 DB 安装包到安装目录**
  - **Pre-check**：安装包路径存在且可读；目标目录可写
  - **Action**：解压（复用 CF-002/CF-003）
  - **Post-check**：关键二进制（如 `yasboot`）存在
  - **Artifacts**：解压目录清单摘要
- **C-005 生成单机 hosts/cluster 配置文件**
  - **Pre-check**：cluster 名称、begin-port、节点 IP/SSH 端口、安装路径参数已提供
  - **Action**：执行“生成配置”的动作（生成 `hosts.toml`、`*.toml`）
  - **Post-check**：两类文件存在且权限符合要求（敏感信息需保护）
  - **Artifacts**：配置文件路径清单与摘要
- **C-006 校验/修改字符集（CHARACTER_SET）（可选）**
  - **Pre-check**：用户显式提供字符集或接受默认；配置文件已生成
  - **Action**：仅修改字符集字段（复用 CF-006）
  - **Post-check**：读取配置确认字段值正确
  - **Artifacts**：配置差异摘要
- **C-007 校验/设置 USE_NATIVE_TYPE（可选）**
  - **Pre-check**：用户显式提供开关值；配置文件已生成
  - **Action**：仅插入/修改该配置项（复用 CF-006）
  - **Post-check**：读取配置确认字段值正确
  - **Artifacts**：配置差异摘要
- **C-008 安装 DB 软件（不含建库）**
  - **Pre-check**：依赖包已满足（B-012，来自 OS 共享模块）；可选 deps 包路径（SSL）已明确
  - **Action**：执行“安装软件”动作（支持带/不带 deps 分支）
  - **Post-check**：相关进程/服务存在（如 yasom/yasagent）
  - **Artifacts**：安装输出摘要与版本信息（如可获取）
- **C-009 创建数据库（deploy）**
  - **Pre-check**：关键不可变参数已确认（字符集/类型开关）；端口未占用（A-004）
  - **Action**：执行建库动作（仅此动作）
  - **Post-check**：集群/实例状态检查通过（primary/standby、normal/open）
  - **Artifacts**：deploy 输出摘要与 status 输出
- **C-010 写入环境变量到安装用户 shell 配置（可选）**
  - **Pre-check**：bashrc 策略已选择（追加/覆盖/独立 env）
  - **Action**：仅写入环境变量片段（复用 CF-006）
  - **Post-check**：在非交互 shell 中验证 `yasboot` 可找到（不要求实际登录）
  - **Artifacts**：变更摘要
- **C-011 DB 登录与基础可用性验证**
  - **Pre-check**：环境变量已生效或使用绝对路径；管理员连接方式已明确
  - **Action**：执行一次最小验证（如能以 sysdba 方式连入）
  - **Post-check**：返回码为 0
  - **Artifacts**：验证输出摘要（脱敏）

### D. 数据库安装与建库（YAC）

> **依赖关系（必须）：** YAC 流程同样应复用 OS 共享模块（见 B 模块/`os prepare`），特别是依赖包、用户/目录权限、网络与防火墙策略等。
>
> **共享存储依赖（必须，按场景启用）：**
> - 若 YAC 使用共享存储且以多路径设备（`/dev/mapper/*`）呈现，则必须在建库前完成并通过：**B-017 ~ B-024**（多路径 + udev）。

- **D-001 校验 YAC 节点清单与数量**
  - **Pre-check**：节点列表已提供且数量 ≥ 2
  - **Action**：检查主机清单格式与连通性（复用 A-002）
  - **Post-check**：所有节点可执行命令
  - **Artifacts**：节点清单与连通报告
- **D-002 校验 SCAN DNS（仅 SCAN 模式）**
  - **Pre-check**：选择 SCAN 模式且 scanname 已提供
  - **Action**：对每个节点执行解析检查（复用 A-003）
  - **Post-check**：每节点解析结果满足预期
  - **Artifacts**：解析结果记录
- **D-003 校验共享盘设备可见性（可选/强建议）**
  - **Pre-check**：system/data/arch 盘路径参数已提供
  - **Action**：检查设备节点存在与权限
  - **Post-check**：所有节点检查通过或失败
  - **Artifacts**：设备检查表
- **D-004 创建 YAC 目录（全节点）**
  - **Pre-check**：目录参数已提供
  - **Action**：创建目录（复用 CF-005）
  - **Post-check**：目录存在
  - **Artifacts**：目录列表记录
- **D-005 解压 DB 安装包（执行节点）**
  - **Pre-check**：执行节点已指定；安装包可读
  - **Action**：解压
  - **Post-check**：关键二进制存在
  - **Artifacts**：解压摘要
- **D-006 生成 YAC hosts/cluster 配置（VIP/SCAN 分支）**
  - **Pre-check**：网络参数（public/inter CIDR、网卡名、vips 或 scanname）与磁盘参数已提供
  - **Action**：仅生成配置文件
  - **Post-check**：配置文件存在且权限符合要求
  - **Artifacts**：配置文件路径清单
- **D-007（可选）写入/调整 YFS 参数**
  - **Pre-check**：用户启用该可选项；配置文件已生成
  - **Action**：仅修改 YFS/redo 等参数字段（复用 CF-006）
  - **Post-check**：读取配置确认字段正确
  - **Artifacts**：差异摘要
- **D-008 安装 DB 软件（全节点）**
  - **Pre-check**：依赖包已满足；可选 deps 包路径明确
  - **Action**：执行安装软件动作
  - **Post-check**：各节点关键进程存在
  - **Artifacts**：安装摘要
- **D-009 创建 YAC 集群数据库（deploy）**
  - **Pre-check**：端口未占用；关键不可变参数已确认
  - **Action**：执行建库动作
  - **Post-check**：`instance_status=open` 且 `database_status=normal`
  - **Artifacts**：status 输出
- **D-010 写入环境变量（含 YASCS_HOME）（可选）**
  - **Pre-check**：已知每节点 YASCS_HOME 路径规则或由参数提供
  - **Action**：仅写入环境变量
  - **Post-check**：验证 `yasboot` 可用
  - **Artifacts**：变更摘要

### E. 新增备库（扩容）

> 目标：在已有主库/集群基础上新增节点并建立备库关系。必须支持失败后的清理与重试策略（不自动执行破坏性清理，需显式选择）。
> 
> **命令**：`yinstall standby`
> 
> **执行流程**：
> 1. 若 `--with-os=true`（默认）：先在备库节点执行 OS 基线配置（B-xxx 步骤，与主库安装时一致）
> 2. 在主库节点执行扩容相关步骤（E-xxx 步骤）
> 3. 扩容完成后在备库节点配置环境变量和自启动

- **E-000 主库连通性检查**
  - **Pre-check**：主库 IP 参数已提供（`--primary-ip`）
  - **Action**：
    - 验证主库 IP 有效性（格式校验）
    - 测试主库 SSH 连通性（使用 `--primary-ssh-user`、`--primary-ssh-password`）
    - 收集主库基本信息（OS 类型、版本、主机名）
  - **Post-check**：主库可达
  - **Artifacts**：主库主机信息摘要

- **E-001 主库状态检查**
  - **Pre-check**：主库 SSH 连通、集群名参数已提供（`--db-cluster-name`）
  - **Action**：
    - 在主库执行 `yasboot cluster status -c <cluster> -d`
    - 验证主库 `database_role=primary`、`database_status=normal`
    - 检查 `yasboot` 命令可用
    - 检查主库解压目录（`--db-stage-dir`）存在
  - **Post-check**：主库运行正常
  - **Artifacts**：主库状态输出

- **E-002 备库节点连通性检查**
  - **Pre-check**：备库目标 IP 参数已提供（`--targets`）
  - **Action**：
    - 验证所有备库节点 IP 有效性
    - 测试所有备库节点 SSH 连通性
    - 收集备库节点基本信息
    - 若 `--with-os=false`：验证备库节点 OS 基线是否已完成（可选校验）
  - **Post-check**：所有备库节点可达
  - **Artifacts**：备库节点信息摘要

- **E-003 主备网络互通检查**
  - **Pre-check**：主库和备库节点均已验证连通
  - **Action**：
    - 从主库 ping 所有备库节点
    - 从所有备库节点 ping 主库
    - 检查关键端口可达性（SSH 端口、数据库端口）
  - **Post-check**：主备网络互通
  - **Artifacts**：网络连通性报告

- **E-004 生成扩容配置文件**
  - **Pre-check**：
    - 集群名、备库节点 IP、用户名/密码、路径参数已提供
    - 主库解压目录存在且 `yasboot` 可用
  - **Action**：
    - 在主库节点执行 `yasboot config node gen` 命令：
      ```
      cd <db_stage_dir> && yasboot config node gen \
        -c <cluster_name> -u <os_user> -p '<os_user_password>' \
        --ip <target_ips> --port 22 \
        --install-path <db_install_path> \
        --data-path <db_data_path> \
        --log-path <db_log_path> \
        --node <node_count>
      ```
    - 验证生成的配置文件：`hosts_add.toml`、`<cluster>_add.toml`
  - **Post-check**：配置文件存在且内容正确
  - **Artifacts**：配置文件路径、内容摘要（脱敏）

- **E-005 安装软件到备库节点（节点扩容）**
  - **Pre-check**：
    - 扩容配置文件已生成（`hosts_add.toml`）
    - 备库节点 SSH 可达
  - **Action**：
    - 在主库节点执行 `yasboot host add` 命令：
      ```
      cd <db_stage_dir> && yasboot host add -c <cluster_name> -t hosts_add.toml [--deps <deps_package>]
      ```
    - 等待软件安装完成
  - **Post-check**：
    - 在所有备库节点检查 `yasagent` 进程运行
    - 执行 `yasboot process yasagent status -c <cluster>` 验证
  - **Artifacts**：安装输出摘要、进程状态

- **E-006 添加备库实例（实例扩容）**
  - **Pre-check**：
    - 软件已安装到备库节点
    - 扩容配置文件存在（`<cluster>_add.toml`）
  - **Action**：
    - 在主库节点执行 `yasboot node add` 命令：
      ```
      cd <db_stage_dir> && yasboot node add -c <cluster_name> -t <cluster>_add.toml
      ```
    - **重要提示**：此命令会触发后台数据同步，命令返回不代表同步完成
  - **Post-check**：
    - 命令返回成功
    - 提示用户数据同步可能仍在进行中
  - **Artifacts**：扩容命令输出

- **E-007 备库同步状态检查**
  - **Pre-check**：实例扩容命令已执行
  - **Action**：
    - 在主库执行 `yasboot cluster status -c <cluster> -d`
    - 检查所有节点状态
    - 验证备库节点：
      - `instance_status=open` 或 `mounted`（同步中）
      - `database_role=standby`
  - **Post-check**：
    - 备库实例已创建
    - 若 `instance_status` 不是 `open`，提示同步进行中
  - **Artifacts**：集群状态输出

- **E-008 备库环境变量配置**
  - **Pre-check**：备库实例已创建
  - **Action**：
    - 在所有备库节点配置环境变量（复用 C-010 逻辑）
    - 根据数据库实例数量决定写入 `.bashrc` 或 `.<port>`
  - **Post-check**：环境变量文件存在
  - **Artifacts**：环境变量配置摘要

- **E-009 备库自启动配置（可选）**
  - **Pre-check**：环境变量已配置
  - **Action**：
    - 在所有备库节点配置 `yashan_monit.sh` 脚本（复用 C-012 逻辑）
    - 配置 systemd 服务（复用 C-013 逻辑）
  - **Post-check**：systemd 服务已启用
  - **Artifacts**：服务状态

- **E-010 扩容完成验证**
  - **Pre-check**：所有扩容步骤已完成
  - **Action**：
    - 最终验证集群状态
    - 在备库节点测试 `yasql / as sysdba` 登录
    - 检查主备同步关系
  - **Post-check**：
    - 所有节点状态正常
    - 备库可登录
  - **Artifacts**：最终状态报告

- **E-011（可选/危险）清理失败扩容产物**
  - **Pre-check**：
    - 必须显式指定 `--force E-011` 或 `--standby-cleanup-on-failure=true`
    - 二次确认（交互模式下）
  - **Action**：
    - 在主库执行 `yasboot node remove -c <cluster> -n <node_id> --clean`
    - 清理备库节点残留文件
  - **Post-check**：清理完成
  - **Artifacts**：清理操作记录
  - **注意**：此为危险操作，仅在扩容失败需要重试时使用

### F. 开机自启（可选模块）

- **F-001 数据库 monit 服务脚本/服务配置（systemd）**
  - 端口/环境文件定位规则必须参数化
  - 需提供“仅生成配置/应用配置/验证服务”三步
- **F-002 YCM systemd 服务配置**
- **F-003 YMP 自启策略**
  - 文档中为“暂不考虑”，工具中可保持为“不提供/可选扩展”

### G. YCM 安装

- **G-001 安装 YCM 依赖包（如 libnsl）（可选）**
  - **Pre-check**：OS/包管理器已探测；包列表已提供
  - **Action**：安装依赖包
  - **Post-check**：确认包已安装
  - **Artifacts**：安装摘要
- **G-002 解压 YCM 安装包到安装目录**
  - **Pre-check**：YCM 包路径存在；安装目录参数已提供（默认 `/opt`）
  - **Action**：解压
  - **Post-check**：`/opt/ycm`（或自定义）存在
  - **Artifacts**：目录清单摘要
- **G-003 修正 YCM 目录属主**
  - **Pre-check**：安装用户存在
  - **Action**：chown
  - **Post-check**：属主正确
  - **Artifacts**：变更摘要
- **G-004 校验/准备 deploy 配置文件**
  - **Pre-check**：deploy 文件路径参数已提供（默认 `/opt/ycm/etc/deploy.yml`）
  - **Action**：仅检查文件存在/可读（不修改）
  - **Post-check**：存在则通过，否则失败
  - **Artifacts**：路径记录
- **G-005 写入/更新 YCM 端口配置**
  - **Pre-check**：端口参数已提供
  - **Action**：仅修改端口字段（复用 CF-006）
  - **Post-check**：读取文件确认端口值正确
  - **Artifacts**：差异摘要
- **G-006 校验端口未占用**
  - **Pre-check**：端口列表已确定
  - **Action**：检查端口占用（复用 A-004）
  - **Post-check**：端口可用
  - **Artifacts**：端口检查表
- **G-007 执行 YCM 初始化部署**
  - **Pre-check**：后端数据库模式已选择（sqlite/yashandb）；若为 yashandb 模式，连接参数与管理员凭据已提供
  - **Action**：执行初始化部署命令（仅此动作）
  - **Post-check**：返回码为 0
  - **Artifacts**：部署输出摘要（脱敏）
- **G-008 验证 YCM 进程存在**
  - **Pre-check**：部署完成
  - **Action**：检查进程（复用 CF-007）
  - **Post-check**：进程存在
  - **Artifacts**：进程列表摘要
- **G-009 验证 YCM 端口监听**
  - **Pre-check**：端口参数已提供
  - **Action**：检查监听（复用 CF-007）
  - **Post-check**：端口处于监听状态
  - **Artifacts**：监听输出摘要
- **G-010（可选）验证 YCM Web 可访问**
  - **Pre-check**：网络策略允许；URL 已知
  - **Action**：执行 HTTP 探测（可选）
  - **Post-check**：返回 200/302 等可接受状态
  - **Artifacts**：HTTP 探测结果

### H. YMP 安装

- **H-001 创建 YMP 用户（如 ymp）**
  - **Pre-check**：用户名与密码策略已提供
  - **Action**：创建用户（复用 CF-005）
  - **Post-check**：用户存在
  - **Artifacts**：用户信息记录
- **H-002 写入 YMP 用户 limits**
  - **Pre-check**：limits 参数已提供
  - **Action**：写入 limits（复用 CF-006）
  - **Post-check**：文件包含预期项
  - **Artifacts**：差异摘要
- **H-003 安装 YMP 依赖包（libaio/lsof 等）**
  - **Pre-check**：包列表已提供
  - **Action**：安装依赖包
  - **Post-check**：确认安装成功
  - **Artifacts**：安装摘要
- **H-004 校验 JDK 版本与架构约束**
  - **Pre-check**：已探测 CPU 架构（A-001）；用户期望 JDK 版本已提供
  - **Action**：仅做校验（ARM 仅允许 11/17）
  - **Post-check**：通过或失败并给出原因
  - **Artifacts**：校验记录
- **H-005 安装 JDK（可选）**
  - **Pre-check**：JDK 包路径存在（rpm 或其他介质）；用户启用“安装 JDK”
  - **Action**：安装
  - **Post-check**：`java -version` 输出符合预期
  - **Artifacts**：版本输出摘要
- **H-006 解压 YMP 软件包**
  - **Pre-check**：YMP zip 路径存在；解压目标目录已提供
  - **Action**：解压
  - **Post-check**：YMP 目录存在（含 `bin/ymp.sh`）
  - **Artifacts**：目录清单摘要
- **H-007 解压 Oracle instantclient（basic）**
  - **Pre-check**：instantclient 包路径存在
  - **Action**：解压
  - **Post-check**：解压目录存在
  - **Artifacts**：目录摘要
- **H-008（可选）解压 Oracle instantclient（sqlplus）并写入环境变量**
  - **Pre-check**：sqlplus 包路径存在；环境文件路径已提供
  - **Action**：仅解压并写 env 文件（复用 CF-006）
  - **Post-check**：PATH/LD_LIBRARY_PATH 文件内容正确（不要求实际登录）
  - **Artifacts**：env 文件差异摘要
- **H-009 执行 YMP 安装初始化**
  - **Pre-check**：DB 安装包路径与 instantclient 路径已提供；安装模式（默认/自定义路径）已选择
  - **Action**：执行 `ymp.sh install`（仅此动作）
  - **Post-check**：返回码为 0
  - **Artifacts**：安装输出摘要（脱敏）
- **H-010 验证 YMP 进程存在**
  - **Pre-check**：安装完成
  - **Action**：检查进程（复用 CF-007）
  - **Post-check**：进程存在
  - **Artifacts**：进程摘要
- **H-011 验证 YMP 端口监听（默认 8090，可配置）**
  - **Pre-check**：端口参数已提供
  - **Action**：检查监听（复用 CF-007）
  - **Post-check**：端口监听存在
  - **Artifacts**：监听输出摘要
- **H-012（可选/危险）清理失败安装产物**
  - **Pre-check**：用户显式选择 cleanup 且二次确认
  - **Action**：仅执行清理动作（删除指定目录/文件）
  - **Post-check**：目标路径不存在
  - **Artifacts**：清理清单与结果

---

## 状态管理与断点续跑（必须）

### RunID 与状态文件

- 每次执行产生唯一 **RunID**
- 必须记录：
  - 执行的目标主机清单与连接信息（脱敏）
  - 每一步的：开始/结束时间、结果、错误摘要、重试次数、产物列表
  - 环境探测结果（OS/架构/关键命令存在性）
- 状态存储位置（建议）：
  - 控制端本地默认目录（可配置）
  - 可选同步到目标端指定目录

### 断点续跑规则

- 失败后可从失败步骤继续（默认）
- 允许用户指定“从某步开始/到某步结束”
- 若输入参数发生变化，工具必须提示：
  - 哪些步骤受影响需要重跑
  - 哪些步骤产物可能不一致

---

## 日志、输出与报告（必须）

### 日志要求

> 本工具必须具备“**屏幕简洁日志 + 文件详细 debug 日志**”的双通道日志体系：屏幕只提示进度与关键结果；debug 文件记录全部细节（命令与输出、关键逻辑分支、重试与耗时等），用于排障与审计。

#### 屏幕日志（Console，简洁）

- **目标**：让用户随时知道“执行到哪一步、在哪台机器、成功/失败、下一步是什么”，避免屏幕刷屏。
- **必须包含字段**：
  - 时间戳、RunID、目标主机标识、StepID、Step 名称、阶段（start/success/fail/skip）、耗时（结束时）
- **输出粒度**：
  - 每个 Step：至少输出开始与结束各一条
  - 失败时：输出错误摘要（不含敏感信息）与建议查看 debug 日志路径
- **禁止**：
  - 默认情况下在屏幕输出完整命令行、stdout/stderr（除非用户显式开启 verbose）

#### Debug 文件日志（必须：全量可审计）

- **目标**：完整记录执行细节，用于复盘、审计与问题定位。
- **必须记录**：
  - **所有在目标端执行的操作系统命令**（包括通过 SSH、本机、sudo 执行）
  - 每条命令的：
    - 执行时间、耗时、执行用户（root/普通用户）、工作目录（如可获取）
    - 命令文本（脱敏后）
    - 返回码（exit code）
    - stdout、stderr（原样记录但需脱敏）
  - Step 生命周期：
    - Pre-check 结果、Action 开始/结束、Post-check 结果
    - 重试次数、每次重试的原因与间隔、最终结果
  - 连接与通道信息（脱敏）：
    - SSH/本机、认证方式（password/key/local）、超时与重连
- **组织方式（必须明确一种）**：
  - 至少支持按 RunID 分目录保存；目录内按主机拆分；每台主机按时间/Step 分段
  - 必须提供“debug 日志文件路径”的固定可预测输出（console 必须提示该路径）

#### 重要逻辑日志（必须：可定位决策过程）

> 除命令级日志外，工具内部的“重要逻辑”必须输出详细 debug 日志，便于理解工具为什么这么做。

- **重要逻辑的定义（至少包括）**：
  - 计划生成与步骤选择：哪些 Step 被纳入/被跳过，原因是什么（tag、依赖、参数、幂等检测结果）
  - OS/版本分支选择：RHEL7 vs RHEL8+、x86_64 vs aarch64 等分支决策
  - 幂等决策：资源已存在时采用 skip/update/fail 的原因与依据
  - 风险开关决策：为何允许/拒绝执行危险步骤（sudoers、防火墙关闭、清理等）
  - 超时/重试策略：触发条件、参数、最终收敛原因
- **关联字段（必须）**：
  - RunID、任务名、主机、StepID、逻辑分支标识（例如 `decision=os_repo_mode=local_iso`）

#### 日志格式与安全（必须）

- **结构化日志**：必须提供结构化格式（如 JSON Lines）以支持机器分析；同时可选提供人类可读格式。
- **统一脱敏**：
  - 口令、密钥、Token、连接串中的密码、sudoers 规则中的敏感片段等必须脱敏
  - 需要定义统一的脱敏规则（例如按 key 名、正则、命令参数位置）
- **完整性与边界**：
  - debug 日志需考虑超大输出（例如安装日志很长），必须有分段/滚动/大小上限策略（由工具内置固定策略实现，不对外暴露参数）

### 报告与产物

- 必须支持导出：
  - 计划（plan）
  - 执行结果汇总（成功/失败步骤清单）
  - 每台主机的关键校验结果（端口、服务状态、版本信息、目录权限等）
  - 生成的配置文件清单（路径与内容摘要，不强制包含全文）

---

## 错误处理、重试与回滚（必须）

### 错误分级

- 可恢复错误：网络闪断、临时锁、端口瞬时占用等（支持自动重试）
- 不可恢复错误：权限不足、参数缺失、版本不兼容、关键资源冲突等（立即失败）

### 重试策略

- 每步采用工具内置重试策略（不对外暴露参数），并在 debug 日志中记录每次重试的原因、间隔与最终收敛原因
- 对“长时间后台动作”（例如备库同步）要有“轮询查询 + 超时”机制

### 回滚/清理策略

- 默认不自动执行破坏性回滚
- 提供显式的 cleanup 步骤（如 E-006、H-007），并要求二次确认

---

## 兼容性与前置条件（必须声明）

### 已验证平台（来自最佳实践）

- Oracle Linux 8、UOS 20、麒麟 V10（需在工具中体现为“已验证”标签）

### 版本兼容性

- 必须要求用户显式提供：
  - 数据库版本（软件包路径/版本号）
  - YCM 版本（软件包路径/版本号）
  - YMP 版本（软件包路径/版本号）
- 工具应提供“兼容性提示位”：
  - 若未提供兼容矩阵数据，至少提示用户需自行确认

---

## 安全与合规（必须）

- 连接与执行要遵循最小权限原则：
  - 能用普通用户完成的步骤不得强制 root
  - 必须提升权限的步骤需清晰标注
- 对 `sudoers` 修改、关闭防火墙等高风险操作：
  - 必须默认不启用，或提供显式开关
  - 必须输出风险提示与回退建议

---

## 交付与验收（建议）

### MVP 验收清单（建议）

- 单机数据库：从空机到可登录、状态正常（含可选 OS 基线）
- YAC（2 节点）：VIP/SCAN 任选一种模式完成安装与状态正常
- 新增备库：从单机主库扩容 1 个备库，状态正常
- YCM：sqlite 后端安装成功并可访问页面；yashandb 后端安装成功（至少连通验证）
- YMP：安装成功、端口与进程正常、可启动/停止
- 失败场景：任意步骤失败后可 resume；危险 cleanup 需二次确认

---

## 待确认问题（评审用）

1. **本机判定规则**：仅支持显式 `--local`，还是自动识别 localhost/127.0.0.1/本机 IP？
2. **是否支持跳板机（bastion）**：若需要，连接模型与参数如何设计？
3. **产物保存策略**：是否需要将生成的 hosts/cluster 配置文件“强制留存”并纳入审计？
4. **默认安全策略**：是否默认禁止修改 sudoers、默认不关闭防火墙，而仅提供端口开放模板？
5. **步骤粒度**：是否需要把“生成配置 / 安装软件 / 建库”进一步拆到更细（例如每条系统配置独立一步）？


