# .freexclaw

`.freexclaw` 用来承载 FreeX Claw 项目自己的工程化约束、提示词、技能和模板资产。

## 目录说明

- `skills/`: 项目级 skills，定义何时触发以及如何执行特定工作流
- `policy/`: 默认工程规范和交付约束
- `prompts/`: 可复用的系统提示或任务提示模板
- `prompts/coding-runtime.md`: 运行时按需注入的短版 coding 摘要
- `templates/`: 常见输出模板，例如交付报告、实施清单

## 当前内容

- `skills/engineering-delivery/SKILL.md`
  - 工程化交付 skill
- `policy/engineering.md`
  - 默认工程实践约束
- `prompts/coding-system.md`
  - 默认 coding system prompt 草案
- `prompts/coding-runtime.md`
  - 低 token 的运行时 coding 摘要
- `templates/delivery-report.md`
  - 交付总结模板

## 使用建议

- 当任务涉及创建、修改、交付代码时，优先遵循 `policy/engineering.md`
- 当需要统一 AI 的编码风格和执行流程时，参考 `prompts/coding-system.md`
- 当需要低成本注入当前代码任务约束时，优先使用 `prompts/coding-runtime.md`
- 当需要输出结果总结时，优先使用 `templates/delivery-report.md`
