# 示例说明

## 示例文章

示例输入文件：[`1/examples/article.sample.json`](1/examples/article.sample.json)

该文件覆盖了以下最小流程：

- 元数据
- 头条
- 一个 section
- 一个 card block
- 结论与 CTA

## 典型调用

### 渲染

```bash
node ./1/src/cli/index.js render ./1/examples/article.sample.json -o ./1/.tmp/article.html --check
```

### 校验

```bash
node ./1/src/cli/index.js validate ./1/examples/article.sample.json
```

### 流程编排

```bash
node ./1/src/cli/index.js pipeline ./1/examples/article.sample.json --output-dir ./1/.tmp/build --dry-run
```

## 作为库调用

```js
import { loadJsonFile } from '../src/core/io.js';
import { renderArticle } from '../src/core/render.js';
import { validateArticle } from '../src/core/validate.js';

const article = await loadJsonFile('./1/examples/article.sample.json');
const html = await renderArticle(article);
const result = await validateArticle(article, { htmlText: html });
```
