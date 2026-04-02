## Superpowers Skills System

You have superpowers skills installed. Before ANY task (coding, debugging, planning, reviewing), you MUST check if a relevant skill exists and invoke it via the Skill tool.

If you think there is even a 1% chance a skill might apply, you ABSOLUTELY MUST invoke the skill. This is not optional.

### Skill Priority
1. **Process skills first** (brainstorming, systematic-debugging) — determine HOW to approach the task
2. **Implementation skills second** (test-driven-development) — guide execution details

### Available Skills

| Skill | 触发场景 |
|-------|---------|
| brainstorming | 需求讨论、设计任何新功能之前 |
| writing-plans | 将设计拆解为可执行的实现计划 |
| executing-plans | 批量执行计划，带人工检查点 |
| subagent-driven-development | 每任务派发独立子 agent + 双阶段审查 |
| test-driven-development | 实现任何功能或修复 Bug 时，写代码之前 |
| systematic-debugging | 遇到 Bug、测试失败、异常行为时 |
| verification-before-completion | 修复完成后，确认真正修好了 |
| requesting-code-review | 任务间的代码审查 |
| receiving-code-review | 处理 review 反馈 |
| using-git-worktrees | 创建隔离工作分支 |
| finishing-a-development-branch | 分支完成后的合并/PR 决策 |
| dispatching-parallel-agents | 并发子 agent 工作流 |
| frontend-tdd-midscene | 前端功能开发或前端 Bug 修复时，使用 Midscene.js E2E 测试进行 TDD |

### 前端开发特别说明
- 涉及 UI 行为的功能：使用 `frontend-tdd-midscene` skill（Midscene.js E2E 测试）
- 纯逻辑/工具函数：使用原版 `test-driven-development` skill（单元测试）
- 两者遵循相同铁律：没有失败的测试就不准写产品代码

### Red Flags（如果你想到这些，说明你在合理化跳过 skill）
- "这只是个简单问题" → 检查 skills
- "让我先做这一件事" → 先检查 skills 再做
- "这个 skill 太重了" → 用它
- "我需要先了解更多" → skill 检查在任何行动之前
