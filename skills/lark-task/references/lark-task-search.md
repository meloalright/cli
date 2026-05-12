# task +search

> **Prerequisites:** Please read `../lark-shared/SKILL.md` to understand authentication, global parameters, and security rules.
>
> **⚠️ Note:** This API must be called with a user identity. **Do NOT use an app identity, otherwise the call will fail.**

Search tasks by keyword and optional filters.

## When not to use `+search`

If the user asks for a list-style task review with no real keyword, prefer the list shortcuts instead of `+search`. Do not turn generic instruction words such as "查找", "总结", "进度", "紧急程度", or date phrases such as "这个月" into `--query`.

Use:

- `task +get-related-tasks` for "我关注的", "我创建的", or "与我相关的" task lists, then apply completion / due-time filtering as needed.
- `task +get-my-tasks` for "分配给我的" or "我负责的" task lists, with `--complete`, `--due-start`, and `--due-end` filters.
- `task +search` when the request includes a task title fragment or domain keyword, for example "发布会", "客户反馈", or "周会纪要".

## Recommended Commands

```bash
# Search by keyword
lark-cli task +search --query "test"

# Search incomplete tasks assigned to specific users
lark-cli task +search --assignee "ou_xxx,ou_yyy" --completed=false

# Search by due time range
lark-cli task +search --query "release" --due "-1d,+7d"

# List tasks assigned to me without a keyword
lark-cli task +get-my-tasks --complete=false --due-start 2026-05-01 --due-end 2026-06-01

# List tasks I follow without a keyword
lark-cli task +get-related-tasks --followed-by-me --include-complete=true
```

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `--query <string>` | No | Search keyword. If omitted, at least one filter must be provided. |
| `--creator <ids>` | No | Creator open_ids, comma-separated. |
| `--assignee <ids>` | No | Assignee open_ids, comma-separated. |
| `--follower <ids>` | No | Follower open_ids, comma-separated. |
| `--completed=<bool>` | No | Filter by completion state. |
| `--due <range>` | No | Due time range in `start,end` form. Each side supports ISO/date/relative/ms input. |
| `--page-token <string>` | No | Page token for pagination. |
| `--page-all` | No | Automatically paginate through all pages (max 40). |
| `--page-limit <int>` | No | Max page limit (default 20). |

## Workflow

1. Build the keyword and filters from the user's request.
2. Execute `lark-cli task +search ...`
3. Report the matched tasks and include the next `page_token` if more results exist.
