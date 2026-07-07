# FreeX Claw

终端里的 AI 编程助手。基于 OpenAI 兼容接口，支持流式对话、文件读写、内建工具调用与多会话管理。

## 安装

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/CooDdk/freexclaw/master/install.sh | sh
```

国内网络可将域名替换为 jsDelivr 镜像：`cdn.jsdelivr.net/gh/CooDdk/freexclaw@master`。

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/CooDdk/freexclaw/master/install.ps1 | iex
```

**手动下载**

从 [Releases](https://github.com/CooDdk/freexclaw/releases) 下载对应平台的二进制，放到任意可执行路径即可。

## 配置

首次运行会打印配置文件位置。编辑并填入 API Key：

| 平台    | 路径                                                      |
| ------- | --------------------------------------------------------- |
| macOS   | `~/Library/Application Support/FREEXCLAW/config.yaml`     |
| Linux   | `~/.config/FREEXCLAW/config.yaml`                         |
| Windows | `%AppData%\FREEXCLAW\config.yaml`                         |

```yaml
api_key: sk-xxx
base_url: https://api.openai.com/v1
model: gpt-4o
```

也可以通过环境变量 `OPENAI_API_KEY` 覆盖。

## 使用

```sh
freexclaw                    # 新建会话
freexclaw --resume <id>      # 恢复历史会话
```

退出时会打印带 `--resume` 的完整命令，直接复制即可回到当前会话。

### 内建命令

| 命令                    | 说明                     |
| ----------------------- | ------------------------ |
| `/read <path>`          | 读取文件到上下文         |
| `/write <path> <text>`  | 写入文件（`-a` 追加）    |
| `/ls [path]`            | 列目录                   |
| `/sessions`             | 会话列表                 |
| `/new` `/clear`         | 新建 / 清空当前会话      |
| `/help` `/quit`         | 帮助 / 退出              |

### 快捷键

| 按键                       | 功能                         |
| -------------------------- | ---------------------------- |
| `Enter`                    | 发送                         |
| `Shift+Enter` / `Ctrl+J`   | 换行                         |
| `Esc`                      | 切换焦点                     |
| `↑` / `↓`                  | 输入框内切换历史             |
| `j` / `k` / `Ctrl+U/D`     | 聊天区滚动                   |
| `Ctrl+C`                   | 连按两次退出                 |

## 特性

- 流式响应，Markdown 与代码高亮
- 多会话，按工作目录隔离
- 内建 `read` / `write` / `ls` 工具，AI 输出代码块可自动落盘
- 天气、汇率、新闻等实时查询走结构化 provider，绕开自由改写搜索词
- SQLite 本地存储，无外部依赖

## 兼容模型

支持任何 OpenAI 兼容接口，包括 OpenAI、DeepSeek、Moonshot 以及本地 Ollama / vLLM / LM Studio。在 `base_url` 和 `model` 里配置即可。

## License

MIT
