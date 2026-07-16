# 快速上手 (Getting Started)

> 面向第一次接触 go-llmx 的开发者：安装 → 首次对话 → 工具/记忆/RAG → 多厂商与容灾，15 分钟跑通全链路。
> 设计原理见 [ARCHITECTURE.md](ARCHITECTURE.md)。

## 0. 环境要求

- Go **1.25+**
- 任一 LLM 厂商 API Key（OpenAI / Anthropic / Google / DeepSeek 等，或本地 Ollama）

## 1. 安装

go-llmx 是多模块仓库（workspace）：**核心契约库 + 按需引入的厂商适配器**。

```bash
# 核心库（对话契约 Model/Embedder、chain/agent/memory/graph 编排、transport 传输）
go get github.com/kamalyes/go-llmx

# 按需引入模型厂商适配器（独立模块，不引入不编译）
go get github.com/kamalyes/go-llmx/adapters/openai      # OpenAI 及所有兼容端点
go get github.com/kamalyes/go-llmx/adapters/anthropic    # Claude
go get github.com/kamalyes/go-llmx/adapters/googleai     # Gemini
go get github.com/kamalyes/go-llmx/adapters/mistral      # Mistral
go get github.com/kamalyes/go-llmx/adapters/cohere      # Cohere
go get github.com/kamalyes/go-llmx/adapters/ollama      # 本地模型

# 按需引入向量库适配器
go get github.com/kamalyes/go-llmx/vectorstores/redisvector
go get github.com/kamalyes/go-llmx/vectorstores/pgvector
go get github.com/kamalyes/go-llmx/vectorstores/milvus
go get github.com/kamalyes/go-llmx/vectorstores/qdrant
```

## 2. 三十行跑通第一次对话

```go
package main

import (
	"context"
	"fmt"

	"github.com/kamalyes/go-llmx"
	"github.com/kamalyes/go-llmx/adapters/openai"
)

func main() {
	// 任何厂商适配器 New 出来的都是同一个 llmx.Model
	model := openai.New("sk-...", openai.WithModel("gpt-4o"))

	ctx := context.Background()

	// 1. 一行生成（最快路径）
	out, _ := llmx.Generate(ctx, model, "用一句话解释 goroutine")
	fmt.Println(out)

	// 2. 结构化消息（多轮）
	resp, _ := model.GenerateContent(ctx, []llmx.Message{
		llmx.System("你是严谨的 Go 工程师"),
		llmx.User("channel 和 mutex 怎么选？"),
	})
	text, _ := llmx.FirstText(resp)
	fmt.Println(text)

	// 3. 请求级参数覆盖（临时换模型/温度，不动构造配置）
	out, _ = llmx.Generate(ctx, model, "hi",
		llmx.WithTemperature(0.2),
		llmx.WithModel("gpt-4o-mini"),
	)
	fmt.Println(out)
}
```

## 3. 流式输出

```go
_, err := model.StreamGenerateContent(ctx, msgs, func(chunk *llmx.Chunk) error {
	fmt.Print(chunk.DeltaText())
	return nil
})
```

`StreamHandler` 只在**首个 chunk 之前**允许容灾切换（见 §7），一旦开始输出绝不重复投递。

## 4. 工具调用（原生 Function Calling）

```go
calc := tool.NewCalculator() // 内置：零依赖表达式求值
sql := tool.NewSQLDatabase(db, 100) // 内置：只读 SELECT 防护

agent := agent.New(model).Tools(calc, sql)
res, _ := agent.Run(ctx, "帮我算 (1250-250)/4，再查 users 表前 3 行姓名")
fmt.Println(res.Answer)
```

自定义工具只需要一个 struct——Schema 自动从 Go 类型反射生成：

```go
weather := tool.Tool{
	Name:        "weather",
	Description: "Query current weather of a city",
	Parameters:  tool.SchemaOf(WeatherArgs{}), // struct 字段 → JSON Schema
	Func: func(ctx context.Context, args string) (string, error) {
		// args 是模型产出的 JSON 字符串
		var w WeatherArgs
		json.Unmarshal([]byte(args), &w)
		return queryWeather(ctx, w.City), nil
	},
}
```

## 5. 记忆与 RAG

### 会话记忆

```go
mem := memory.NewBuffer()             // 全量缓冲
mem := memory.NewWindow(20)           // 滑动窗口（最近 N 条）
mem := memory.NewSummary(model, 4)    // 摘要记忆 + 最近 4 轮原文

conv := &chain.ConversationChain{Model: model, Memory: mem}
out, _ := conv.Run(ctx, "我叫 Kamalyes")
out, _ = conv.Run(ctx, "我叫什么？") // 记得住
```

摘要记忆的压缩是**显式的**——`mem.Condense(ctx)` 由你决定时机（对比某些库藏在内部隐式触发）：

```go
if len(mem.Messages()) > 50 {
	_ = mem.Condense(ctx) // 超窗轮次 LLM 压缩进摘要，失败自动回滚保原文
}
```

### RAG 检索问答

```go
// 加载 → 切分（LoadAndSplit 一步完成）→ 嵌入 → 入库
docs, _ := documentloaders.NewHTML(file).LoadAndSplit(ctx,
	textsplitter.NewTokenSplitter(512, 64))
texts := make([]string, len(docs))
for i, d := range docs {
	texts[i] = d.PageContent
}
vecs, _ := embedder.EmbedDocuments(ctx, texts)
_ = store.AddDocuments(ctx, docs, vecs)

// 检索问答链（stuff 模式；retriever 为任一 llmx.Retriever 实现）
qa := chain.NewRetrievalQA(model, retriever)
out, _ := qa.Run(ctx, "go-llmx 的错误哨兵有哪些？")

// 多轮版：自动改写"它的并发模型？"这类指代问题后再检索
conv := chain.NewConversationalRetrievalQA(model, retriever).WithMemory(mem)
```

## 6. 切换厂商（无损）

业务代码只依赖 `llmx.Model` 接口——**换厂商 = 改构造那一行**：

```go
model := openai.New(key)                      // → 换成：
model := anthropic.New(key)                   // 其余代码零改动
model := ollama.New("http://localhost:11434") // 或本地模型
```

OpenAI 兼容端点（DeepSeek/OpenRouter/Groq/vLLM）一个适配器通吃：

```go
model := openai.New(key,
	openai.WithBaseURL("https://api.deepseek.com"),
	openai.WithModel("deepseek-chat"))
```

同一适配器换模型走请求级覆盖：`llmx.WithModel("deepseek-reasoner")`。

## 7. 容灾路由（Failover / Random / RoundRobin）

```go
// 依序故障切换：主模型不可达/5xx/空响应时自动切下一个
routed := llmx.NewFailover(primary, backup)

// 随机分流（多 Key 采样）/ 轮询均衡（配额 A/B）
routed = llmx.NewRandomModel(a, b, c)
routed = llmx.NewRoundRobinModel(a, b)

// routed 就是 llmx.Model —— chain/agent/memory 全生态直接用
conv := &chain.ConversationChain{Model: routed, Memory: mem}
```

Embedder 有对应的 `llmx.NewEmbedderFailover`——**限定同模型多 Key 容灾**，切换不改变向量空间，存量索引不失效。

**切换规则**：`ErrProviderUnavailable / ErrAPIServerError / ErrEmptyResponse` 触发切换；参数错误、认证失败换后端也救不了，直接透传。流式只在首 chunk 前切换，避免重复输出。

## 8. 错误处理

所有可预期错误都是**哨兵错误**，`errors.Is` 精确匹配：

```go
out, err := llmx.Generate(ctx, model, "hi")
switch {
case errors.Is(err, llmx.ErrEmptyResponse):
	// 空响应 → 值得切换后端重试
case errors.Is(err, llmx.ErrProviderUnavailable):
	// 网络不可达 → 指数退避
case errors.Is(err, llmx.ErrInvalidRequest):
	// 参数错误 → 检查代码，别重试
}
```

## 9. 下一步

- [ARCHITECTURE.md](ARCHITECTURE.md) —— 每个模块的设计取舍与性能数据
- [README.md](README.md) —— 模块导航与架构图
- [adapters/openai/examples](./adapters/openai/examples/) —— 可运行的端到端样例（streamtools 为 mock SSE，无需密钥直接跑）
