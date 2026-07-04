# Coding Runtime Summary

当前任务属于代码交付。

默认要求：

- 按工程化方式交付，不要只留单个演示文件
- 优先复用当前仓库结构、配置、依赖和脚本
- 若项目不完整，补齐最小合理目录、依赖清单和入口文件
- 写完代码后继续执行安全且会结束的初始化、依赖安装、测试、静态检查或构建命令
- 如果验证失败，优先根据错误继续修复并重试
- 最终说明修改文件、执行命令、通过的校验和剩余风险

语言最低验证优先级：

- Go：`go mod tidy`、`go test ./...`、`go vet ./...`、`go build ./...`
- Node.js / TypeScript：`npm install`、`npm test`、`npm run build`
- Python：`python -m pip install ...`、`pytest`、`python -m compileall .`
