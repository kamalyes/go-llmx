# 架构与原理 (Architecture)

> go-llmx 的设计取舍、核心机制与性能证据。上手请先读 [GETTING-STARTED.md](GETTING-STARTED.md)。

## 1. 总览

```
                     ┌─────────────────────────────────────────┐
                     │            你的业务代码                   │
                     └───────────────┬─────────────────────────┘
                                     │ 只依赖接口 (llmx.Model / llmx.Embedder)
        ┌────────────────────────────┼────────────────────────────┐
        │ 编排层                      │ 适配层                     │
        │ chain (LLM/对话/检索/路由)   │ adapters/{openai,anthropic..} │
        │ agent (原生 ReAct 并行工具)  │ embeddings/{5 家}          │
        │ memory (缓冲/窗口/摘要)      │ vectorstores/{5 库}        │
        │ graph (知识图谱记忆)         │ tool (计算器/SQL)           │
        │ failover (容灾路由)         │ documentloaders/textsplitter│
        └────────────────────────────┴────────────────────────────┘
```

**一条铁律**：编排层 100% 面向接口编程，厂商只是构造期注入的实例。因此换厂商、加容灾路由对业务代码是零改动的。

## 2. 契约层（根包 llmx）

| 契约 | 能力 | 关键设计 |
|---|---|---|
| `Model` | GenerateContent / StreamGenerateContent | 统一消息 `[]Message`、`Option` 请求级覆盖 |
| `Embedder` | EmbedDocuments / EmbedQuery | 文档与查询双形态，向量空间一致性由调用方保证 |
| `VectorStore` | Init / AddDocuments / SimilaritySearch | 统一 `Filter`（map 元数据等值过滤）|
| `Memory` | Add / Messages / Clear | 无 error 签名——记忆不是失败点 |
| `Chain` | Run(ctx, input) | 最小编排单元，自由组合 |

**错误哨兵**：`ErrInvalidRequest / ErrEmptyResponse / ErrProviderUnavailable / ErrAPIServerError / ErrInvalidVectors / ErrInvalidResponse / ErrUnsupportedOperation / ErrNoModels / ErrNoEmbedders`。适配器把协议错误映射到哨兵，编排层只做 `errors.Is` 分支。

## 3. 适配器：能力协商而非能力强制

不同厂商能力不同，go-llmx 的策略是**"支持的消费、不支持的静默忽略"**：

- `WithN(5)` 在不支持多候选的厂商上返回 1 个，不报错
- `WithThinking(...)` 仅推理型适配器消费
- 多模态 `ImagePart` 只有视觉模型有意义

语义无损的代价是能力降级而非运行时失败——对比"参数必须全部生效否则 panic"的设计，这在容灾路由场景（§6）是唯一正确的语义：切后端不能因为参数差异而炸。

## 4. 编排层原理

### 4.1 chain：模板缓存与显式压缩

所有文档链（stuff/refine/mapreduce/retrieval QA）共享 **`sync.Map` 缺省模板缓存**：模板懒解析一次，后续 `Run` 命中缓存。

> 基准（20 篇 × 200 字符文档）：渲染 17456 → 9775 ns/op（**1.79x**），50 → 8 allocs/op（**-84%**）。对照组即"每次 Run 重新解析"的形态——langchaingo 的实现方式。

`ConversationalRetrievalQA` 的**首轮跳过改写**：无历史时直接检索原始问题，省一次 LLM 调用；有历史时先 LLM 改写指代（"它的并发模型？"→"What is Go's concurrency model?"）再检索。

`Summary` 记忆的压缩是**显式 `Condense(ctx)`**，且**失败回滚**：LLM 报错时轮次队列原样恢复，原文零丢失可重试。

### 4.2 agent：原生工具调用 ReAct

不走"文本协议解析 ReAct"（langchaingo 老路径），直接用**厂商原生 Function Calling**：

- 一轮内多个工具调用**并行执行**（`sync.WaitGroup`），缩短多工具往返时延
- 工具注册表 `sync.Once` 预计算，循环内零重复构造
- `Result` 类型安全自愈：JSON 解析失败的结果自动降级为原始文本而非报错中断

### 4.3 token 切分：流式字节扫描

`TokenSplitter` 近似 token 预算（rune/4，与 `memory.ApproxTokens` 同一规则），切点优先词边界。实现是**流式字节扫描**：`utf8.DecodeRuneInString` 原地解码 + `text[byteA:byteB]` 切块共享底层。

> 基准（100KB 中英混排）：667769 → 2160 B/op（**-99.7%**），73 → 7 allocs/op（**-90%**），1.48x 提速。

对比 langchaingo `TokenTextSplitter` 依赖 tiktoken-go（词表文件 + 逐 token 查表），go-llmx 零词表零 CGO。

### 4.4 graph：三元组邻接表

知识图谱 `Triple{Subject, Predicate, Object}`：去重 map + 出入边双向索引；BFS 可达集（depth 截断）与最短路用于 `GraphMemory` 实体召回——一跳全量 + 二跳截断（`recallHop2Limit`）。

### 4.5 内存向量库：预归一化 + Top-K 堆 + 并发分片

余弦检索的三个结构性优化：

1. **写入时预归一化**：余弦只看方向，存量向量的范数写入后永不改变——归一化副本随写入缓存，检索每条从 3 乘 3 加降为 1 乘 1 加
2. **Top-K 最小堆**：size=k 堆 O(n log k) 替代全量排序 O(n log n)，同分按写入序号稳定淘汰
3. **并发分片**：超 4096 条按 `GOMAXPROCS`（上限 16 worker）分片并行，片内局部堆合并全局堆；小库走单线程避免 goroutine 开销反噬

> 基准（50k 条 × 256 维 topK=10）：149.5 → 13.6 ms/op（**10.95x**），8.77 → 0.068 MB/op（**-99.2%**）；小库（2k 条）单线程路径 2.29x、-95.3%。对照组即优化前形态（每次全量算范数 + `sort.SliceStable` 全排序）——langchaingo 内存库的结构。

等价性由对照测试守恒：同分冲突、零向量、维度不等（回退逐条余弦保留截断语义）、filters、大库分片路径，新旧输出逐位一致。

## 5. 向量库适配层

五个实现（redisvector / pgvector / milvus / qdrant + transport 复用）共同策略：

1. **数据格式对齐 langchaingo**：表结构、字段名、payload 键完全一致，存量数据平滑读写
2. **批量写入**：pgx.Batch / Milvus 行式 insert / Qdrant batch upsert——一次网络往返
3. **检索裁剪**：outputFields 只取正文与元数据，向量 blob 不回传
4. **Schema 自动建**：`Init` 检查集合存在性，缺失时按维度/度量自动创建

## 6. 容灾路由

```
Failover          依序切换（主备）
RandomModel       随机分流（多 Key 采样）
RoundRobinModel   轮询均衡（配额 A/B）
EmbedderFailover  Embedder 专属：同模型多 Key
```

- **切换条件**：`failoverOf(err)` = 不可达 / 5xx / 空响应；参数与认证错误透传（换后端救不了）
- **流式语义**：首 chunk 前失败可切换；一旦有产出，错误直接透传——已收内容不可回放，避免重复输出
- **EmbedderFailover 的语义边界**：跨模型切换会改变向量空间，导致存量索引与查询向量不可比——那是索引重建任务，不是路由组件的事

## 7. 多模块仓库（workspace）

```
go-llmx/               # 核心契约 + 编排（零厂商依赖）
├── adapters/openai    # 独立 go.mod：OpenAI 及兼容端点
├── adapters/…         #   ···每个适配器独立模块（6 家）···
├── embeddings/…       # 5 家嵌入适配器
├── vectorstores/…     # 5 个向量库适配器
└── go.work            # workspace 聚合
```

**不引入不编译**：用 pgvector 就不会拉 Milvus 依赖。核心模块保持零厂商 SDK 依赖。

## 8. 测试策略

- **零外部依赖**：适配器全用 fake 接口 / httptest mock 服务器注入，覆盖协议编解码、索引构建、检索过滤全路径
- **每组件独立 `_test.go`**：与被测文件同名同目录
- **基准即证据**：性能主张（模板缓存、字节扫描）都有 `-bench` 数据与对照组
- **SQLite 测试**：modernc.org/sqlite 纯 Go 无 CGO 内存库

## 9. 与 langchaingo 的关键差异

| 维度 | langchaingo | go-llmx |
|---|---|---|
| 模板 | 每次 Run 重新解析 | sync.Map 缓存（1.79x）|
| token 切分 | tiktoken 词表依赖 | 流式字节扫描（-99.7% 分配）|
| 内存向量库 | 每次全量算范数 + 全排序 | 预归一化 + 堆 + 分片（10.95x）|
| agent 工具循环 | 逐个串行 | 同轮并行执行 |
| 摘要记忆 | 隐式触发、失败静默 | 显式 Condense、失败回滚 |
| 模块边界 | 单模块全量依赖 | workspace 按需引入 |
| SQL 工具 | 写读皆可 | 只读防护 + 行数截断 |
| Schema 生成 | kin-openapi 传递依赖 | 反射自研零依赖 |
