---
name: git-conventional-commit
description: "Use this skill at the end of every repository task that changes files, before the final response, to review the worktree, run scope-appropriate validation based on the project's technology stack, stage only intended files, and create a Conventional Commits git commit. Trigger this skill whenever a task is completed, finished, done, or ready to commit. Keywords: git commit, git add, git push, conventional commits, commit changes, stage files, save changes, task completion, finishing implementation, documentation changes, refactoring, fixing bugs, testing, final response, ending turn."
---

# Git Conventional Commit

在每次任务收尾时，把已完成变更收敛成一个可审查、可追溯、符合 Conventional Commits 的提交。除非用户明确要求不要提交，任务结束前必须使用本 skill。

## 收尾流程

1. **读取当前状态**：
   - `git status --branch --short`
   - `git diff --stat`
   - `git diff --check` (检查是否有悬挂空格、合并冲突标记等)

2. **按项目技术栈动态运行验证**：
   识别当前工作区的主要配置文件并运行相应的构建、测试或格式化检查：
   - **Rust (Cargo.toml)**: 运行 `cargo check`；如果是核心热路径或业务修改，运行 `cargo test`。
   - **Go (go.mod)**: 运行 `go test ./...` 校验受影响的包或全量测试。
   - **Node.js (package.json)**: 运行适用的 `npm/pnpm/yarn test` 或 `typecheck` / `lint` 校验。
   - **Python (pyproject.toml/requirements.txt)**: 运行对应的 pytest / linter。
   - **其他技术栈**: 运行该技术栈对应的标准构建、单元测试或格式化工具。
   - **仅文档 / 规则变更**: 至少运行 `git diff --check`，无需进行复杂的编译 and 测试验证。

3. **审查 diff，确认没有混入**：
   - 用户未要求的文件。
   - 敏感信息（如 `.env`、密码、密钥、个人凭证等）。
   - 本地配置、运行态临时数据、构建生成目录或测试报告。
   - 未经验证的依赖锁文件、编译产物或编辑器临时文件。

4. **只暂存本次任务相关文件**：
   - 使用显式路径 `git add <path...>` 进行精确暂存。
   - 避免直接使用 `git add .` 或 `git add -A`，除非任务明确覆盖整个工作树且已逐行审查所有 diff。

5. **生成 Conventional Commits 信息并提交**：
   - 执行 `git commit -m "<message>"`。

6. **提交后复查**：
   - `git status --branch --short`
   - `git log -1 --oneline`

## 提交信息规则

格式：

```text
<type>(<scope>): <subject>
```

允许的 `type`：

- `feat`：新增用户可见能力、模块、API、页面或核心业务脚本。
- `fix`：修复缺陷、错误行为或验证失败。
- `refactor`：不改变外部行为的代码结构调整、重构。
- `docs`：仅文档、README、AGENTS.md、注释或设计说明的变更。
- `test`：新增、调整或修复测试用例。
- `build`：构建系统、依赖管理、编译选项、Docker、CI 或流水线配置变更。
- `chore`：例行维护、脚本清理、辅助工具升级、仓库治理。
- `style`：格式化、空白字符修正、纯样式调整（不改变代码逻辑）。
- `perf`：提升性能或优化资源占用的变更。
- `ci`：持续集成配置与脚本变更（如 GitHub Actions、GitLab CI）。

`scope` 使用受影响的稳定目录、模块名或特性名称，例如：
- `core`, `api`, `web`, `config`, `docs`, `agents`, `skill`, `types`, `auth`, `infra` 等。
- 应尽量精简，且为英文小写。

`subject` 规则：
- 使用英文祈使句（如 `add feature` 而非 `added feature` 或 `adds feature`）。
- 首字母小写，结尾不加句号。
- 长度尽量控制在 72 个字符以内。

示例：
- `docs(agents): update dynamic backpressure guidelines`
- `feat(auth): support token-based session expiration`
- `fix(infra): prevent resource leak in pool destruction`
- `refactor(core): decouple connection manager ports`

## 自动提交边界

必须自动提交：
- 本次任务确实修改了文件且需要保存。
- 验证已按风险范围完成，或无法运行的原因（如环境缺口）已经明确记录。
- diff 只包含本次任务相关内容。

不得自动提交：
- 用户明确要求不要提交、只查看、只分析或只给出方案。
- 工作区存在与本次任务无关的用户本地修改，且无法可靠分离。
- 自动化验证失败，且失败不是已知可接受的环境问题。
- diff 中包含敏感数据（密钥）、本地配置或临时生成文件。
- 当前处于未完成的合并（merge）、变基（rebase）或冲突未解决状态。

当符合“不得自动提交”的条件时，请停止提交操作，并在最终回复中说明原因、已验证的内容以及建议的后续操作。

## 最终回复要求

完成提交后，回复中必须包含：
- 提交的 commit hash 与完整的 commit message。
- 主要修改的文件列表。
- 已运行的验证命令及结果。
- 如果仍有未暂存或未提交的变更，清晰说明原因。
