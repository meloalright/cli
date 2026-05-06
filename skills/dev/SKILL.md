---
name: dev
version: 0.3.0
description: "eval-search 交付 Harness：借鉴 lkkcli /dev 的生命周期约束，把搜索评测目标落成可执行、可复盘、可修正的阶段计划。"
metadata:
  requires:
    bins: ["node", "git"]
---

# dev — eval-search 交付 Harness

本 skill 只负责把 lkkcli `/dev` 的生命周期控制迁移到本仓库，不改变 `/eval-search` 的目标：评测 `lark-cli` 搜索能力，产出盲测轨迹、Judge 评分、归因和 Optimizer 可消费的报告。

## 定位

- `/eval-search` 是业务目标层：定义 Executor / Judge / Optimizer 隔离、评分、污染控制和 PR 生成。
- `scripts/harness-runner.ts` 是状态执行层入口，直接通过 Node 的 TS type stripping 执行。
- `.harness/plan.example.json` 是本仓库默认计划：用 lkkcli 风格的 `prepare -> understand -> plan -> act -> verify -> retrospect` 包住 eval-search。

不要把这个 skill 扩展成通用研发流水线；通用需求、部署、MR 和 CI 编排属于 lkkcli `/dev`。这里的交付标准仍然围绕搜索评测。

## 硬约束

1. **目标不漂移**：plan 的 `target.skill` 必须是 `eval-search`。
2. **输入先声明**：loader profile、executor profile、subset/dataset-file、eval run id 必须写进 `inputs`。
3. **生命周期可检查**：plan 必须声明 `lifecycle.stage_order`，并开启 `constraints.enforce_stage_order`。
4. **角色隔离保留**：Loader、Executor、Judge、Optimizer 的输入边界必须写进 `constraints.role_isolation`。
5. **TS 替代 JS**：runner、setup runner、evidence collector 只保留 `.ts`，不要再生成或维护同名 `.js`。
6. **产物契约显式化**：rubric、Executor/Judge/Optimizer prompt、TS 入口必须列入 `artifacts`。
7. **失败可恢复**：失败 step 必须输出 `self_correction`；能自动 correction 的写进 plan，不能自动处理的给出 next action。

## 标准入口

先运行本仓库默认计划，确认 eval-search 的静态契约和本地门禁都成立：

```bash
node --experimental-strip-types scripts/harness-runner.ts --plan .harness/plan.example.json --format json
```

运行产物写入：

```text
.harness/runs/<run-id>/
  plan.json
  contract.json
  stage_shape.json
  events.ndjson
  stages/<stage-id>.json
  summary.json
```

只有当 `summary.status == "passed"` 时，才继续执行真实 `/eval-search run` 或 `/eval-search propose-pr`。

## 生命周期语义

### Prepare

确认 repo 状态、分支、dirty 文件和本地工具可用性。`lark-cli` 缺失不直接阻断静态门禁，但真实评测前必须补齐。

### Understand

读取并确认 `/eval-search` 的核心契约：盲测、三角色隔离、rubric、污染控制。这个阶段不接触评测集答案。

### Plan

确认 deterministic setup 和 evidence collector 可调用，并明确本轮使用的 loader/executor profile、subset、dataset-file 策略。

### Act

检查或执行会产出 eval-search 运行材料的代码路径：dataset setup、pollution preflight、executor evidence collection。

### Verify

运行递进式门禁：TypeScript check、runner syntax、eval-search 脚本 syntax、skill format。真实评测完成后，还要检查 `tests/eval-search/runs/<run-id>/summary.json` 和 regression 结果。

### Retrospect

沉淀本轮的污染 token、失败归因、泛化改动声明和下一轮 correction。若需要新增经验，优先更新 `skills/eval-search/**` 或 `tests/eval-search/**`，不要散落到临时笔记。

## 收尾标准

最终回复用户前检查最新 summary：

```bash
node --experimental-strip-types scripts/harness-runner.ts --plan .harness/plan.example.json --format json
```

如果 `summary.status != "passed"`，不能声称完成；必须给出 `summary.contract_failures` 和 `summary.failed_steps[*].next_actions`。
