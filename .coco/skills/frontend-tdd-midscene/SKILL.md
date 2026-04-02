---
name: frontend-tdd-midscene
description: "Use when implementing any frontend feature or fixing frontend bugs. Enforces TDD using Midscene.js for E2E testing with Playwright. Trigger when: developing UI components, pages, user flows, form interactions, frontend routing, or any user-facing feature. Also trigger when user mentions frontend testing, e2e testing, UI testing, or Midscene."
---

# Frontend TDD with Midscene.js

## Overview

Write the E2E test first using Midscene.js + Playwright. Watch it fail. Write minimal frontend code to pass.

**Core principle:** If you didn't watch the E2E test fail in the browser, you don't know if it tests the right user behavior.

## When to Use

**Always for frontend development:**
- New pages / routes
- New UI components with user interaction
- Form flows and validations
- User journey features (login, checkout, search)
- Bug fixes that affect UI behavior

**Exceptions (ask your human partner):**
- Pure styling changes with no behavior
- Static content updates
- Build/config changes

## Tech Stack

- **Test Runner**: Playwright
- **AI Automation**: Midscene.js (`@midscene/web`)
- **Language**: TypeScript

## Project Setup

If not already configured, set up the test infrastructure:

### Install Dependencies

npm install @midscene/web playwright @playwright/test --save-dev

### Configure playwright.config.ts

export default defineConfig({
  testDir: './e2e',
  timeout: 90 * 1000,
  reporter: [
    ["list"],
    ["@midscene/web/playwright-reporter", { type: "merged" }]
  ],
});

### Create Test Fixture (e2e/fixture.ts)

import { test as base } from '@playwright/test';
import type { PlayWrightAiFixtureType } from '@midscene/web/playwright';
import { PlaywrightAiFixture } from '@midscene/web/playwright';

export const test = base.extend<PlayWrightAiFixtureType>(
  PlaywrightAiFixture({
    waitForNetworkIdleTimeout: 2000,
  }),
);

### Configure Midscene Model (环境变量)

export MIDSCENE_MODEL_BASE_URL="your-model-service-url"
export MIDSCENE_MODEL_API_KEY="your-api-key"
export MIDSCENE_MODEL_NAME="your-model-name"

推荐使用 Doubao-Seed-1.6-Vision 或 Qwen3-VL 等多模态模型。

## TDD 流程：RED-GREEN-REFACTOR

### RED - 写失败的 E2E 测试

使用 Midscene.js 的自然语言 API 描述用户行为和预期结果：

<Good>
import { expect } from '@playwright/test';
import { test } from './fixture';

test('user can search products and see results', async ({
  ai, aiQuery, aiAssert, aiInput, aiTap, aiWaitFor,
}) => {
  // 用户行为：在搜索框输入关键词
  await aiInput('无线耳机', '搜索框');

  // 用户行为：点击搜索按钮
  await aiTap('搜索按钮');

  // 等待结果加载
  await aiWaitFor('搜索结果列表已加载', { timeoutMs: 5000 });

  // 断言：验证用户能看到搜索结果
  await aiAssert('页面上至少显示了一个商品卡片');

  // 提取数据验证
  const items = await aiQuery(
    '{title: string, price: number}[], 获取搜索结果中的商品标题和价格'
  );
  expect(items?.length).toBeGreaterThan(0);
});
</Good>

<Bad>
// 不要用传统选择器写测试再让 Midscene 验证
test('search works', async ({ page }) => {
  await page.locator('#search-input').fill('耳机');
  await page.locator('#search-btn').click();
  // 这不是 Midscene 的用法，没有利用 AI 视觉能力
});
</Bad>

**Midscene 核心 API：**

| API | 用途 | 示例 |
|-----|------|------|
| `aiInput(text, target)` | 在指定元素中输入文本 | `aiInput('用户名', '用户名输入框')` |
| `aiTap(target)` | 点击指定元素 | `aiTap('提交按钮')` |
| `aiAssert(assertion)` | AI 视觉断言 | `aiAssert('页面显示登录成功提示')` |
| `aiQuery(schema)` | 从页面提取结构化数据 | `aiQuery('{name: string}[]')` |
| `aiWaitFor(condition)` | 等待页面满足条件 | `aiWaitFor('加载完成')` |
| `aiScroll(params)` | 滚动页面 | `aiScroll({ scrollType: 'untilBottom' })` |

### Verify RED - 运行测试，确认失败

**必须执行，不可跳过。**

npx playwright test ./e2e/your-test.spec.ts

确认：
- 测试失败（不是报错）
- 失败原因是功能尚未实现（不是选择器错误或环境问题）
- 失败信息清晰描述了缺失的行为

**测试通过了？** 说明你在测试已有行为，修改测试。

### GREEN - 写最小前端代码

写刚好让 E2E 测试通过的前端代码。

**规则：**
- 只实现测试要求的行为
- 不添加"以后可能用到"的功能（YAGNI）
- 不美化超出测试要求的 UI

### Verify GREEN - 运行测试，确认通过

npx playwright test ./e2e/your-test.spec.ts

确认：
- 当前测试通过
- 其他测试仍然通过
- Midscene 报告中截图显示正确的 UI 状态

**查看可视化报告：** 测试完成后会生成 HTML 报告，包含每一步的截图和 AI 推理过程，在 `midscene_run/report/` 目录下。

### REFACTOR - 清理代码

测试通过后：
- 提取可复用组件
- 优化命名和结构
- 清理临时代码

保持测试绿色。不添加新行为。

## E2E 测试编写规范

### 以用户视角描述

<Good>
// 描述用户看到和做的事情
await aiInput('张三', '姓名输入框');
await aiTap('下一步按钮');
await aiAssert('页面显示了地址填写表单');
</Good>

<Bad>
// 不要描述 DOM 结构
await aiTap('#next-btn');
await aiAssert('div.address-form is visible');
</Bad>

### 一个测试一个用户流程

<Good>
test('用户可以完成注册流程', async ({ ai, aiInput, aiTap, aiAssert }) => {
  await aiInput('test@example.com', '邮箱输入框');
  await aiInput('Password123!', '密码输入框');
  await aiTap('注册按钮');
  await aiAssert('页面显示注册成功的提示信息');
});
</Good>

<Bad>
test('注册页面所有功能', async ({ ai }) => {
  // 测试太多东西：输入验证 + 注册 + 登录跳转 + ...
});
</Bad>

### 使用 aiQuery 做数据驱动断言

const userInfo = await aiQuery(
  '{name: string, email: string, role: string}, 获取页面上显示的用户信息'
);
expect(userInfo.role).toBe('admin');

## 与原版 TDD Skill 的关系

本 skill 是 `test-driven-development` skill 在前端 E2E 场景的具体实现。核心铁律不变：

- **NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST**
- 先写了代码？删除它。从测试开始重来。
- 手动在浏览器里测过了？不算。写自动化 E2E 测试。

当需要写单元测试（如工具函数、hooks）时，回退到原版 `test-driven-development` skill。当涉及用户可见的 UI 行为时，使用本 skill。

## 常见问题

| 问题 | 解决方案 |
|------|---------|
| Midscene AI 识别不到元素 | 使用更具体的自然语言描述，如"页面顶部的蓝色搜索按钮"而非"按钮" |
| 测试运行太慢 | 启用 Midscene 缓存：设置 `MIDSCENE_CACHE=true` 环境变量 |
| 弹窗/Toast 消失太快 | 使用 `aiWaitFor` 等待出现，或调整 `waitForNetworkIdleTimeout` |
| 多模态模型不稳定 | 优先使用推荐模型（Doubao-Seed-1.6-Vision / Qwen3-VL），避免纯文本模型 |

## Verification Checklist

在标记任务完成前：

- [ ] 每个用户可见的新行为都有 E2E 测试
- [ ] 每个测试都先看到它失败了
- [ ] 失败原因是功能缺失（不是环境问题）
- [ ] 写了最小代码让测试通过
- [ ] 所有测试通过
- [ ] Midscene 报告截图显示正确的 UI
- [ ] 测试用自然语言描述用户行为（不是 DOM 选择器）
- [ ] 边界情况和错误状态有覆盖
