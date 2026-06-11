# Privacy Vault CLI (pvctl)

AI 编程助手的隐私数据替换工具。在工具输出（`read_file`、`grep`、`run_shell_command` 等）到达模型之前，自动将敏感信息替换为占位符。

## 安装

```bash
go build -o pvctl .
```

## 快速开始

### 1. 配置替换规则

复制示例配置并填入你的隐私数据：

```bash
cp .privaterc.toml.example .privaterc.toml
```

编辑 `.privaterc.toml`：

```toml
[[rules]]
match = "张三"          # 要替换的敏感内容
mode = "token"          # 匹配模式
placeholder = "<NAME>"  # 替换后的占位符

[[rules]]
match = "13800138000"
mode = "exact"
placeholder = "<PHONE>"

[[rules]]
match = "zhangsan@example.com"
mode = "exact"
placeholder = "<EMAIL>"
```

### 2. 注册 hook

一条命令同时注册到 Claude Code 和 Qwen Code：

```bash
pvctl hook install
```

注册后，AI 工具的输出会自动经过隐私替换再发送给模型。

### 3. 验证

```bash
pvctl hook-test
```

输出示例：
```
=== 替换后的文本 ===
[pvctl 隐私替换结果]
<NAME>的手机号是<PHONE>，邮箱<EMAIL>
```

## 两种部署模式

### 模式一：CLI 管道（默认）

每次 hook 触发时调用 `pvctl hook-post` 子进程。

```
AI 工具执行 → hook 触发 → pvctl hook-post（读 stdin、替换、写 stdout）→ 模型收到
```

**优点**：零配置，无需常驻进程
**缺点**：每次调用需加载 gse 中文分词词典（~2s）

```bash
# 注册（默认 cli 模式）
pvctl hook install

# 或显式指定
pvctl hook install --mode cli
```

### 模式二：HTTP 常驻服务

启动一个 HTTP 服务，hook 通过 HTTP POST 调用。gse 词典只加载一次。

```
AI 工具执行 → hook 触发 → HTTP POST → pvctl serve（已加载词典）→ 模型收到
```

**优点**：词典只加载一次，后续请求毫秒级响应
**缺点**：需要先启动服务

```bash
# 终端 1：启动服务
pvctl serve --port 9876

# 终端 2：注册 hook（http 模式）
pvctl hook install --mode http --port 9876
```

## 命令参考

```
pvctl <命令> [选项]

命令:
    replace     从 stdin 读取内容，替换隐私数据后输出到 stdout
    serve       启动 HTTP 服务，作为 hook 端点
    hook-post   PostToolUse hook 命令（CLI 管道模式）
    hook        为 Claude Code / Qwen Code 注册 hook
    hook-test   发送模拟 hook 请求，验证隐私替换效果
```

### `pvctl hook`

```
pvctl hook install   [--claude] [--qwen] [--mode http|cli] [--port N]
                      [--claude-settings PATH] [--qwen-settings PATH]
pvctl hook uninstall [--claude] [--qwen]
                      [--claude-settings PATH] [--qwen-settings PATH]
pvctl hook show      [--claude] [--qwen]
                      [--claude-settings PATH] [--qwen-settings PATH]
```

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `--claude` | 操作 Claude Code 配置 | 两者都操作 |
| `--qwen` | 操作 Qwen Code 配置 | 两者都操作 |
| `--mode` | `http` 或 `cli` | `cli` |
| `--port` | HTTP 模式端口 | `9876` |
| `--claude-settings PATH` | 指定 Claude Code settings.json 路径 | `~/.claude/settings.json` |
| `--qwen-settings PATH` | 指定 Qwen Code settings.json 路径 | `~/.qwen/settings.json` |

### `pvctl hook-test`

```
pvctl hook-test [--mode cli|http|direct] [--text "测试文本"] [--port N]
```

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `--mode` | `cli` 通过子进程、`http` 通过 HTTP 服务、`direct` 直接调用引擎 | `cli` |
| `--text` | 含敏感数据的测试文本 | `张三的手机号是13800138000，邮箱zhangsan@example.com` |
| `--port` | HTTP 模式端口 | `9876` |

## 匹配模式

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| `token` | gse 中文分词后精确匹配 | 中文姓名（"张三"不匹配"张三丰"） |
| `exact` | 全词匹配（ASCII 字符边界） | 英文标识符、手机号、邮箱、API Key |
| `substring` | 子串替换 | 地址、长文本片段 |

## Hook 工作原理

pvctl 通过 Qwen Code / Claude Code 的 **PostToolUse hook** 机制工作：

```
用户提问 → 模型调用工具(read_file/grep/Bash)
         → 工具返回结果（可能含隐私数据）
         → PostToolUse hook 拦截结果
         → pvctl 替换隐私数据
         → 替换后的结果发送给模型
```

注册 hook 后，`~/.qwen/settings.json` 或 `~/.claude/settings.json` 中会写入：

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Bash|ReadFile|Grep|Glob|WriteFile|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/pvctl hook-post",
            "name": "pvctl-privacy-replace",
            "timeout": 30
          }
        ]
      }
    ]
  }
}
```

## 配置文件

默认查找顺序：`.privaterc.toml` → `.pvctl.toml` → `pvctl.toml`

可通过 `--config` 指定其他路径。

完整配置示例见 `.privaterc.toml.example`。

## 测试

```bash
go test ./...
```
