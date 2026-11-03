# Special Judge（SPJ）与测试数据同步

## 一、启用 SPJ

题目编辑页把「比对模式」选为 **Special Judge 程序判定**，再选择语言（C++ / C）并填入判定程序源码即可。

- 后台接口：`POST /api/admin/v1/problems`、`PUT /api/admin/v1/problems/:id`，字段 `compare_mode: "spj"`、`spj_language: "cpp" | "c"`、`spj_code`（源码字符串）
- 用户端「我的题目」：同样支持 `compare_mode`、`spj_language`、`spj_code`
- 切回 `default` / `exact` / `float` 时不需要清空 SPJ 代码，代码会被保留但不再使用
- 启用 SPJ 但源码为空会返回「启用 SPJ 时需要填写 SPJ 程序」

## 二、判定程序约定

评测机把判定程序编译成 `/work/spj`，每个测试点执行：

```
/work/spj <输入文件> <标准输出文件> <选手输出文件>
```

- 三个参数依次是：测试点输入（`/box/<index>.in`）、标准答案（`/box/<index>.out`）、选手输出（`/work/case<index>.out`）
- 退出码 `0` 表示该测试点通过（得分按测试点分值），非 `0` 或超时（15 秒）表示未通过
- 标准输出/错误输出会截断后作为测试点的提示信息返回给选手
- 判定程序在沙箱内编译与运行，和选手程序一样没有网络、工作目录只读（`/tmp` 可写）

## 三、测试数据同步

- 测试数据只保存在主站，导入题目时不再逐题推送评测节点
- 后台上传、改分、重排后仍会立即尝试推送一次，失败的节点记在题目的同步状态里
- 主站每 2 分钟检查一次「有测试数据但还没有同步（或测试数据比上次同步更新）」的题目，批量推送到所有评测节点
- `problems.testdata_synced_at` 记录上次推送成功的时间
