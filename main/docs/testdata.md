# 测试数据格式与评分

本文说明题目的测试数据文件命名、分值配置和子任务（subtask）计分规则。题目导入（`docs/problem-import.md`）与后台「题目 → 测试数据」页都遵循同一套规则。

## 一、文件命名

一组测试点就是一对同名的输入、输出文件：

```
sample.in
sample.out
```

- 文件名不限，`1.in/1.out`、`sample.in/sample.out`、`big_case.in/big_case.out` 都可以
- 输出文件后缀可以是 `.out` 或 `.ans`，两者等价
- 去掉扩展名后必须成对：只有 `sample.in` 没有 `sample.out` 会报错
- 名字大小写不敏感，`Case1.in` 与 `case1.out` 视为同一组
- 顺序按文件名自然排序：`2` 排在 `10` 前面，数字段按数值大小比较，其余按字符比较

导入或上传后，文件会按排序结果统一落盘为 `1.in`、`1.out`、`2.in`……评测机按序号取数据，所以原始文件名不会影响评测。

## 二、分值

默认把 100 分平均分给所有**独立测试点**（不参与子任务的测试点），除不尽的余数从**最后一个**测试点往前补。

要精确控制分值和分组，就在测试数据目录里放一个可选的 `subtasks.json`：

```json
{
  "scores": {
    "sample": 20,
    "case1": 10
  },
  "subtasks": [
    {
      "name": "小数据",
      "score": 30,
      "cases": ["case2", "case3"]
    }
  ]
}
```

- `scores`：按测试点文件名（去掉扩展名）指定分值，可只写一部分
- `subtasks`：子任务列表，`name` 必填、`score` 该子任务的总分、`cases` 参与该子任务的测试点文件名
- 一个测试点只能属于一个子任务，重复引用会报错
- 引用了不存在的测试点会报错
- 子任务分 + 独立测试点默认分不能超过 100：超出时独立测试点的默认分按 0 处理（不会出现总分超过 100）

编辑页下方的「测试数据」面板可以直接粘贴同一份 JSON（不需要重新上传数据文件），两者内容一致时以页面上提交的为准。

## 三、计分规则

| 测试点类型 | 单个测试点分值 | 怎么得分 |
| --- | --- | --- |
| 独立测试点 | `scores` 里指定的值，或默认均分 | 该测试点通过就得对应分数 |
| 子任务内测试点 | 0 | 单个测试点不给分 |
| 子任务 | `score` | 组内全部测试点都通过才得该子任务的分，任一失败或缺失则 0 分 |

总分 = 所有独立测试点得分 + 所有通过的子任务得分。例如 `subtasks.json` 里「小数据」30 分包含 `case2`、`case3`，只有这两个都 AC 才拿到 30 分；只过 `case2` 得 0 分。

也可以在编辑页的测试数据面板里逐个测试点填分数：改完点「保存分值」即可，不需要重新上传文件；分数怎么改都不会超过 100 分（子任务分先扣）。

评测结果里每个测试点都会带上它所属的子任务编号（`group`），子任务本身不单独出现在结果列表中，分数在最终得分里体现。

## 四、上限与错误提示

| 项 | 限制 |
| --- | --- |
| 测试点数量 | ≤ `MAX_TESTCASES`，默认 100 |
| 单个文件大小 | ≤ `MAX_TESTDATA_MB`，默认 512MB |

常见错误：

| 报错 | 原因 |
| --- | --- |
| 未找到测试数据文件：任意文件名的一对 .in 与 .out（或 .ans）即可，例如 sample.in 与 sample.out | 没有成对的输入输出文件 |
| 测试点 sample 缺少输入文件（sample.in） | 只有同名输出，没有输入 |
| 测试点 sample 缺少输出文件（sample.out 或 sample.ans） | 只有同名输入，没有输出 |
| 测试点数量 N 超过上限 100 | 超过 `MAX_TESTCASES` |
| subtasks.json 格式错误: … | JSON 语法错误 |
| 子任务 小数据 引用了不存在的测试点 case9 | `cases` 里有不存在的名字 |
| 测试点 case2 被多个子任务引用 | 同一个测试点出现在两个子任务里 |

## 五、导入与接口

- 打包导入：`problem.md` 与测试数据放在同一个 zip 里即可，`subtasks.json` 一并放入，具体包结构见 `docs/problem-import.md`
- 后台上传：`POST /api/admin/v1/problems/:id/testdata`，表单字段 `files`（多个）、`scores`（JSON 数组，可选）、`subtasks`（`subtasks.json` 的内容字符串，可选）
- 用户上传个人题测试数据：`POST /api/v1/my/problems/:id/testdata`，字段相同
- 读取：`GET /api/admin/v1/problems/:id/testdata`、`GET /api/v1/my/problems/:id/testdata` 返回 `{ total_score, subtasks, items }`，`items` 每项含 `index`、`name`、`group_id`、`score`、`input_size`、`output_size`、`input_hash`、`output_hash`

- 改分：`PUT /api/admin/v1/problems/:id/testdata/scores`（body `{"scores":[{"index":1,"score":20}]}`），用户端同路径为 `PUT /api/v1/my/problems/:id/testdata/scores`
- 重排：`POST /api/admin/v1/problems/:id/testdata/reorder`（body `{"order":[3,1,2]}`，`order` 是原测试点序号的新顺序），服务端会按新顺序重写磁盘上的 `1.in/1.out…` 并同步子任务引用；用户端为 `POST /api/v1/my/problems/:id/testdata/reorder`
- 测试数据只保存在主站：导入题目时不再逐题推送到评测节点，后台每 2 分钟把「测试数据有更新还没同步」的题目同步一次（`testdata_synced_at` 记录上次成功同步时间）；上传/改分后仍会立即尝试同步一次，失败的节点会记在题目的同步状态里
