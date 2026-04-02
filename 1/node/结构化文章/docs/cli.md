# CLI 说明

CLI 入口：[`1/src/cli/index.js`](1/src/cli/index.js)

## render

将 article JSON 渲染为 HTML。

```bash
node ./1/src/cli/index.js render ./1/examples/article.sample.json -o ./1/.tmp/article.html --check
```

参数：
- `input`：article JSON 路径
- `-o, --output`：输出 HTML 路径
- `--check`：渲染后顺带校验

## validate

校验 article JSON。

```bash
node ./1/src/cli/index.js validate ./1/examples/article.sample.json
```

参数：
- `input`：article JSON 路径
- `--html`：可选，附带已渲染 HTML 一起校验

## pipeline

运行完整流程。

```bash
node ./1/src/cli/index.js pipeline ./1/examples/article.sample.json --output-dir ./1/.tmp/build --dry-run
```

参数：
- `input`：article JSON 路径
- `--output-dir`：输出目录
- `--dry-run`：仅本地渲染与校验

## 退出码

- `0`：成功
- `1`：失败或校验未通过
