# FREE-X

> Terminal AI Programming Assistant

FREE-X 是一款终端 AI 助手，支持自然语言交互、文件读写、代码生成、文档编写等功能。在任何工作目录下运行，每个目录拥有独立的会话列表，提供安全可靠的数据持久化。

## ✨ 功能特性

### 🤖 智能对话
- **自然语言交互**：与 AI 进行流畅的自然语言交流
- **流式输出**：实时显示 AI 回复，支持 Markdown 渲染
- **多会话管理**：创建、切换、重命名、删除会话
- **工作目录隔离**：不同工作目录拥有独立的会话列表

### 📁 文件操作
- **文件读取**：读取任意文件内容并加入对话上下文
- **文件写入**：创建、覆盖、追加文件内容
- **目录浏览**：列出当前或指定目录的文件
- **相对路径**：所有操作基于当前工作目录

### 🧠 Agent 模式
- **自然语言驱动**：用自然语言命令 AI 操作文件
- **ReAct 模式**：AI 自动思考并调用工具完成任务
- **自动提取**：AI 输出代码块时自动保存到文件

### 💾 数据安全
- **SQLite 存储**：事务安全、并发保护的数据库存储
- **工作空间隔离**：不同目录独立会话，互不干扰
- **自动保存**：程序退出时自动保存所有会话

## 🚀 快速开始

### 安装

下载对应平台的预编译二进制文件：

| 平台 | 架构 | 文件 |
|------|------|------|
| Windows | amd64 | `free-x-windows-amd64.exe` |
| macOS | amd64 | `free-x-darwin-amd64` |
| macOS | arm64 | `free-x-darwin-arm64` |
| Linux | amd64 | `free-x-linux-amd64` |
| Linux | arm64 | `free-x-linux-arm64` |

### 运行

```bash
# 在任意工作目录下运行
cd /path/to/your/project
./free-x
```

**Windows 用户**：双击 `free-x-windows-amd64.exe` 运行

### 配置

首次运行需要配置 API Key，支持以下两种方式：

#### 方式一：环境变量（推荐）

```bash
# Linux / macOS
export OPENAI_API_KEY=sk-xxx

# Windows PowerShell
$env:OPENAI_API_KEY = "sk-xxx"
```

#### 方式二：配置文件

配置文件路径：

- Windows: `%AppData%\free-x\config.yaml`
- macOS: `~/Library/Application Support/free-x/config.yaml`
- Linux: `~/.config/free-x/config.yaml`

配置示例：

```yaml
api_key: sk-xxx
base_url: https://api.openai.com/v1
model: gpt-4o
system_prompt: ""
```

## 📖 使用方法

### 基本对话

直接输入问题即可与 AI 对话：

```
你好，帮我生成一个 Python 脚本
```

### 自然语言操作文件

FREE-X 支持自然语言驱动的文件操作，AI 会自动调用工具完成任务：

```
帮我读取 main.go 文件并分析代码结构
```

```
帮我生成一个 README.md 文件，包含项目介绍和使用说明
```

```
列出当前目录的所有文件
```

### 命令方式操作

除了自然语言，也可以使用命令进行精确操作：

```bash
/read main.go              # 读取文件
/read package.json         # 读取 JSON 文件

/write test.md 这是内容    # 写入文件（覆盖）
/write -a log.txt 追加内容 # 追加内容到文件

/ls                        # 列出当前目录
/ls src/                    # 列出指定目录

/sessions                  # 查看所有会话
/open 1                    # 进入第一个会话
/rename Python 脚本开发    # 重命名当前会话

/new                       # 新建对话
/clear                     # 清空当前对话
/save                      # 保存对话

/help                      # 显示帮助
/quit                      # 退出
```

## ⌨️ 快捷键

| 快捷键 | 功能 |
|--------|------|
| `Enter` | 发送消息 |
| `Shift+Enter` / `Ctrl+J` | 换行 |
| `Esc` | 切换焦点（输入框 ↔ 聊天区） |
| `↑` / `↓` | 输入框：切换历史消息 |
| `j` / `k` | 聊天区：上下滚动 |
| `Ctrl+u` / `Ctrl+d` | 聊天区：翻页 |
| `g` / `G` | 聊天区：跳到顶部/底部 |
| `Ctrl+C` | 退出或停止生成 |

## 🤝 支持的模型

FREE-X 支持所有 OpenAI 兼容接口的模型：

- **OpenAI**：gpt-4o, gpt-4, gpt-3.5-turbo
- **DeepSeek**：deepseek-chat, deepseek-coder, deepseek-v4-pro
- **Claude**：通过兼容接口访问
- **本地模型**：Ollama, vLLM, LM Studio 等

## 📄 License

MIT License

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📧 联系

如有问题或建议，欢迎反馈。

---

**FREE-X** — 让 AI 编程更简单