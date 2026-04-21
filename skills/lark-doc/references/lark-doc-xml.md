基于 HTML 子集的 XML 格式描述飞书文档内容。

# 一、标准 HTML 标签
p, h1-h9, ul, ol, li, table, thead, tbody, tr, th, td, blockquote, pre, code, hr, img, b, em, u, del, a, br, span 语义不变

# 二、扩展标签速查表
## 块级标签
|标签|说明|关键属性|
|-|-|-|
| `<title>` | 文档标题（每篇唯一）| `align` |
| `<checkbox>` | 待办项| `done="true"\|"false"` |

## 容器标签
|标签|说明|关键属性|
|-|-|-|
| `<callout>` | 高亮框，子块仅支持文本、标题、列表、待办、引用 | `emoji`(默认 bulb), `background-color`, `border-color`, `text-color` |
| `<grid>` + `<column>` | 分栏布局，各列 width-ratio 之和为 1 | `width-ratio` |
| `<whiteboard>` | 嵌入画板 | `type`: `mermaid` \| `plantuml` \| `blank` |
| `<pre>` | （代码块，内含 `code`）| `lang`, `caption` |
| `<figure>` | 视图容器 | `view-type` |
| `<bookmark>` | 书签链接 | `<bookmark name="标题" href="https://..."></bookmark>`，必传 name 和 href |

## 行内组件
| 标签 | 说明 | 关键属性 |
|-|-|-|
| `<cite type="user">` | @人 | `<cite type="user" user-id="userID"></cite>` |
| `<cite type="doc">` | @文档 | `<cite type="doc" doc-id="docx_token"></cite>` |
| `<latex>` | 行内公式 | `<latex>E = mc^2</latex>` |
| `<img>` | 图片（可独立成块或内联） | `<img width="800" height="600" caption="说明" name="图.png" href="http 或 https"/>` |
| `<source>` | 文件附件（可独立成块或内联） | `<source name="报告.pdf"/>` |
| `<a type="url-preview">` | 预览卡片 | `<a type="url-preview" href="...">标题</a>` |
| `<button>` | 操作按钮 | `background-color`,src,必须包含 `action=OpenLink|DuplicatePage|FollowPage` |
| `<time>` | 提醒 | `必包含 expire-time、notify-time=毫秒时间戳、should-notify=true|false` |

## 文本块通用属性
- `align` — `"left"`|`"center"`|`"right"`（适用于 p / h1-h9 / li / checkbox）
- 有序列表项用 `seq="auto"` 自动编号

# 三、资源块

文档中可嵌入外部资源块（属于容器标签的特殊形式），需要额外语法创建：

- `<img>` — `<img href="https://..."/>` 上传网络图片
- `<whiteboard>` — `<whiteboard type="blank"></whiteboard>` 空白；`<whiteboard type="mermaid|plantuml">内容</whiteboard>` 带内容；
- `<sheet>` — `<sheet type="blank"></sheet>` 空白；`<sheet sheet-id="SID" token="TOKEN"></sheet>` 复制已有
- `<task>` — `<task task-id="GUID"></task>`，必传 task-id（任务 guid）
- `<chat_card>` — `<chat_card chat-id="CHAT_ID"></chat_card>`，必传 chat-id
- bitable、base_ref、synced_reference、synced_source、okr — 不可创建，仅支持移动

# 四、块级复制与移动

## 移动（block_move_after）
支持**所有**块类型（块级标签、容器标签、行内组件、资源块），使用 `docs +update --command block_move_after --block-id "<锚点>" --src-block-ids "id1,id2"`。

## 复制（block_copy_insert_after）
- **基础标签**（块级标签、容器标签、行内组件）：均支持复制
- **资源块**：仅 img、source、whiteboard、sheet、chat_card 支持复制；task、bitable、base_ref、synced_reference、synced_source、okr 不支持复制

使用 `docs +update --command block_copy_insert_after --block-id "<锚点>" --src-block-ids "id1,id2"`。

> 详见 [lark-doc-update.md](lark-doc-update.md)。

# 五、补充规则

## 富文本样式嵌套顺序
- 行内样式标签必须按以下固定顺序嵌套（外 → 内），关闭顺序严格反转：`<a> → <b> → <em> → <del> → <u> → <code> → <span> → 文本内容`

## 列表分组
- 连续同类型列表项自动合并为一个 `<ul>` 或 `<ol>`
- 嵌套子列表放在 `<li>` 内部
- 新增列表项必须包在 `<ul>` 或 `<ol>` 内：
   ```xml
   <ul>
     <li>第一项</li>
     <li>第二项</li>
   </ul>
   ```


## 表格扩展
标准 HTML table 结构不变，扩展点：
- `<colgroup>` / `<col>` 定义列宽，紧跟 `<table>` 之后：`<col span="2" width="100"/>`
- `<th>` / `<td>` 增加 `background-color` 和 `vertical-align`（top | middle | bottom）
- 有表头时第一行在 `<thead>` 用 `<th>`，其余在 `<tbody>` 用 `<td>`
- 合并单元格仅起始格输出 `colspan` / `rowspan`，被合并的格不出现
- 单元格内可嵌套文本 + 行内格式标签（`<b>`、`<em>`、`<a>`、`<span>` 等）；跨行文字用 `<br/>`。**不**在单元格内嵌套 `<p>`、`<ul>`、`<callout>` 等块级容器。

> **结构性变更**（加行 / 删列 / 合并 / 列宽 / 单元格样式）走 [`lark-doc-table-ops.md`](lark-doc-table-ops.md) 的 `table_*` 指令；本节只讲如何用 XML 表达表格。

## 表格画廊 — 五种常用场景

### 1. 简单 3×3 表格（基线）

```xml
<table>
  <thead><tr><th>列A</th><th>列B</th><th>列C</th></tr></thead>
  <tbody>
    <tr><td>1</td><td>2</td><td>3</td></tr>
    <tr><td>4</td><td>5</td><td>6</td></tr>
  </tbody>
</table>
```

### 2. 表头横向合并（`colspan`）— 做跨列标题

```xml
<table>
  <thead>
    <tr><th colspan="3" background-color="light-blue"><b>季度报表</b></th></tr>
    <tr><th>项目</th><th>负责人</th><th>进度</th></tr>
  </thead>
  <tbody>
    <tr><td>迁移 A</td><td>张三</td><td>80%</td></tr>
    <tr><td>迁移 B</td><td>李四</td><td>30%</td></tr>
  </tbody>
</table>
```

### 3. 纵向合并（`rowspan`）— 做分组标签

```xml
<table>
  <thead><tr><th>类别</th><th>子项</th><th>数量</th></tr></thead>
  <tbody>
    <tr><td rowspan="2">饮料</td><td>咖啡</td><td>12</td></tr>
    <tr><td>茶</td><td>8</td></tr>                              <!-- 注意：本行无"饮料"单元格，被上方合并 -->
    <tr><td>小吃</td><td>饼干</td><td>20</td></tr>
  </tbody>
</table>
```

### 4. 单元格样式 — 背景色 + 垂直对齐

```xml
<table>
  <tbody>
    <tr>
      <td background-color="light-green" vertical-align="top">左上</td>
      <td background-color="light-yellow" vertical-align="middle">中</td>
      <td background-color="light-red" vertical-align="bottom">右下</td>
    </tr>
    <tr>
      <td background-color="rgb(230,240,255)">自定义 RGB 背景</td>
      <td>默认样式</td>
      <td><b>加粗</b> + <span text-color="blue">蓝字</span></td>
    </tr>
  </tbody>
</table>
```

### 5. 列宽控制（`<colgroup>` + `<col>`）

```xml
<table>
  <colgroup>
    <col width="80"/>
    <col width="200"/>
    <col width="120"/>
  </colgroup>
  <thead><tr><th>编号</th><th>说明（宽列）</th><th>状态</th></tr></thead>
  <tbody>
    <tr><td>1</td><td>本列宽 200px，用于较长文本</td><td>✅</td></tr>
  </tbody>
</table>
```

> 也可用 `<col span="2" width="100"/>` 让相邻多列共享一个宽度定义。

## 表格属性一览

| 标签 | 属性 | 取值 | 默认 / 说明 |
|------|------|------|-------------|
| `<table>` | — | — | 容器，无特殊属性 |
| `<colgroup>` | — | — | 可选容器，紧跟 `<table>`，包含一组 `<col>` |
| `<col>` | `span` | 正整数 | 覆盖几个连续列，默认 `1` |
| `<col>` | `width` | 像素整数 | 列宽（px），建议 `60 ~ 600` |
| `<thead>` / `<tbody>` | — | — | 语义分组；第一 `<tr>` 放 `<thead>` 的视觉表头 |
| `<tr>` | — | — | 无特殊属性 |
| `<th>` / `<td>` | `colspan` | 正整数 | 横向合并跨度，默认 `1`，上限 `100` |
| `<th>` / `<td>` | `rowspan` | 正整数 | 纵向合并跨度，默认 `1`，上限 `100` |
| `<th>` / `<td>` | `background-color` | 命名色（`light-{基础色}` / `medium-gray`）或 `rgb(r,g,b)` / `rgba(r,g,b,a)` | 默认无背景 |
| `<th>` / `<td>` | `vertical-align` | `top` \| `middle` \| `bottom` | 默认 `top` |

权威实现参考：`docx_engine/biz/export/xml/token/table.go:27-99`。

# 六、美化系统
- 颜色优先使用命名色，也可写 `rgb(r,g,b)` / `rgba(r,g,b,a)`。**基础色（7 色）**：gray, red, orange, yellow, green, blue, purple
  | 属性 | 支持的命名色 |                                                                                                                                                                                                        
  |-|-|
  | 文字颜色 `<span text-color>` | 基础色 |
  | 高亮框字色 `<callout text-color>` | 基础色 |
  | 高亮框边框 `<callout border-color>` | 基础色 |                                                                                                                                                                                 
  | 文字背景 `<span background-color>` | 基础色 + `light-{色}` + `medium-gray` |                                                                                                                                                   
  | 高亮框填充 `<callout background-color>` | `gray` + `light-{色}` + `medium-{色}` |                                                                                                                                              
  | 单元格背景 `<th/td background-color>` | 同文字背景 |                                                                                                                                                                           
  | 按钮背景 `<button background-color>` | 同文字背景 |
- 常用 emoji： 💡(默认)✅❌⚠️📝❓❗👍❤️📌🏁⭐

# 七、**重要规则**
## 转义规则：标签本身 **禁止转义**，只有标签内部的文本内容才需要转义

**错误** ❌：`&lt;p&gt;内容&lt;/p&gt;`（把标签也转义了）
**正确** ✅：`<p>A &amp; B 的对比：1 &lt; 2</p>`（标签保持原样，文本中的 `&` 和 `<` 才转义）

转义字符表：
- `<` → `&lt;`
- `>` → `&gt;`
- `&` → `&amp;`
- `\n`（换行符） → `<br/>`


## 八、完整示例

```xml
<title>文档标题</title>

<h1>一级标题</h1>

<p><b>加粗文本</b>，<span text-color="green">绿色文本</span></p>

<callout emoji="💡" background-color="light-yellow" border-color="yellow">
  <p>高亮框内容，子块仅支持文本/标题/列表/待办/引用</p>
</callout>

<checkbox done="true">已完成事项</checkbox>
<checkbox done="false">未完成事项</checkbox>

<grid>
  <column width-ratio="0.5">
    <p>左栏</p>
  </column>
  <column width-ratio="0.5">
    <p>右栏</p>
  </column>
</grid>

<table>
  <colgroup><col span="2" width="120"/></colgroup>
  <thead><tr><th background-color="light-gray">表头</th><th background-color="light-gray">表头</th></tr></thead>
  <tbody><tr><td>单元格</td><td>单元格</td></tr></tbody>
</table>

<p><cite type="doc" doc-id="DOC_TOKEN"></cite> <cite type="user" user-id="USER_ID"></cite></p>

<ol><li seq="auto">第一项</li><li seq="auto">第二项</li></ol>

<p><a type="url-preview" href="https://example.com">链接标题</a></p>

<p><latex>E = mc^2</latex></p>

<pre lang="go" caption="示例"><code>fmt.Println("hello")</code></pre>

<hr/>

<source name="文件名.pdf"/>
<img src="IMG_TOKEN" width="800" height="400" caption="说明" name="图.png"/>
<img href="https://example.com/photo.png"/>

<button action="OpenLink" src="https://example.com">按钮文字</button>

<time expire-time="1775916000000" notify-time="1775912400000" should-notify="false">时间戳毫秒</time>

<cite type="citation"><a href="https://example.com">引文标题</a></cite>
<bookmark name="书签标题" href="https://example.com"></bookmark>

<task task-id="TASK_GUID"></task>
<chat_card chat-id="CHAT_ID"></chat_card>
```