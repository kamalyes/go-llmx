# langchaingo → go-llmx 迁移指引

> **诚实结论先行**：go-llmx **不是 drop-in 替换**，而是**低摩擦迁移**。
> 核心概念（模型/消息/流式/工具/链/记忆/嵌入/检索）全部一一对应，
> 但方法签名与组装方式有刻意差异（收窄、显式化、去回调层）。
> 常规业务代码迁移量集中在：**消息构造、流式调用形态、chain 调用签名**三处，其余多为改 import + 构造器。

## 迁移总览

| 迁移项                          | 摩擦度    | 说明                                                                                |
| ---------------------------- | ------ | --------------------------------------------------------------------------------- |
| 客户端构造                        | 🟢 低   | options 模式 → 显式参数 + With 选项，字段一一对应                                                |
| 非流式对话                        | 🟢 低   | `GenerateContent` 同名同形，仅消息类型不同                                                    |
| 调用选项                         | 🟢 低   | `WithTemperature/WithMaxTokens/WithModel` 名字基本一致                                  |
| embeddings                   | 🟢 低   | `EmbedDocuments/EmbedQuery` 完全同名（构造器形态不同）                                         |
| textsplitter                 | 🟢 低   | 同构，仅构造器名变化                                                                        |
| 消息构造                         | 🟡 中   | `llms.MessageContent{Role, Parts}` → `llmx.Message{Role, Content []Part}`          |
| 流式对话                         | 🟡 中   | `WithStreamingFunc([]byte)` 选项 → `StreamGenerateContent(handler)` 独立方法（结构化 Chunk） |
| 工具调用                         | 🟡 中   | 自写循环 / agents MRKL → `RunToolLoop` 一行驱动                                           |
| chains                       | 🟡 中   | `Call(map→map)` → `Run(string→string)`，收窄为单值进出                                    |
| vectorstores                 | 🟡 中   | `WithEmbedder` 注入向量化 → 显式两步（先 EmbedQuery 再检索）                                        |
| 错误处理                         | 🟢 低   | 两边均为哨兵/错误码体系，`errors.Is` 直接映射（见第九步）                                                  |
| agents / callbacks / loaders | 🔴 无对应 | 见[无对应清单](#无对应清单)，均有替代方案                                                           |

***

## 第一步：客户端构造

```go
// langchaingo
import "github.com/tmc/langchaingo/llms/openai"

llm, err := openai.New(
    openai.WithToken(os.Getenv("OPENAI_API_KEY")),
    openai.WithModel("gpt-4o-mini"),
    openai.WithBaseURL("https://api.deepseek.com/v1"), // 兼容端点
    openai.WithHTTPClient(&http.Client{Timeout: 120 * time.Second}),
)

// go-llmx
import lcopenai "github.com/kamalyes/go-llmx/adapters/openai"

c := lcopenai.New(
    os.Getenv("OPENAI_API_KEY"),                    // 密钥为显式首参
    lcopenai.WithModel("gpt-4o-mini"),
    lcopenai.WithBaseURL("https://api.deepseek.com/v1"),
    lcopenai.WithHTTPClient(&http.Client{Timeout: 120 * time.Second}),
)
```

**差异说明**：

- 密钥从 `WithToken` 选项改为构造函数首参（必填语义显式化；本地 Ollama 传空串）
- `New` 不再返回 error（网络客户端构造无可失败操作）
- 厂商切换：langchaingo 换包 + 换构造器；go-llmx 同样换包，但 OpenAI 兼容协议一个适配器覆盖 DeepSeek/OpenRouter/Groq/vLLM，只换 `WithBaseURL`
- 运行期热更：`SetBaseURL/SetModel/SetAPIKey` 访问器直接切换（langchaingo 需重建客户端）

## 第二步：消息构造

```go
// langchaingo（MessageContent + Role 枚举 + Parts 切片）
import "github.com/tmc/langchaingo/llms"

messages := []llms.MessageContent{
    llms.TextParts(llms.ChatMessageTypeSystem, "你是简洁的助手"),
    llms.TextParts(llms.ChatMessageTypeHuman, "你好"),
    llms.TextParts(llms.ChatMessageTypeAI, "你好！有什么可以帮你？"),
    llms.TextParts(llms.ChatMessageTypeHuman, "介绍一下 Go"),
}

// go-llmx（构造函数 + Part 多态）
import llmx "github.com/kamalyes/go-llmx"

messages := []llmx.Message{
    llmx.System("你是简洁的助手"),
    llmx.User("你好"),
    llmx.Assistant("你好！有什么可以帮你？"),
    llmx.User("介绍一下 Go"),
}
```

**差异说明**：

- 结构对等：`llms.MessageContent{Role, Parts}` → `llmx.Message{Role, Content []Part}`，两边都是"角色 + 多态 Part 切片"
- 纯文本快捷方式：langchaingo 用 `llms.TextParts(role, s...)`；go-llmx 用 `llmx.System/User/Assistant(text)` 构造函数（少一层角色枚举）
- 多模态对照：`llms.TextPart/llms.ImageURLPart/llms.BinaryPart` → `llmx.TextPart/ImagePart`（Part 多态语义一致）
- 工具结果：langchaingo 用 part 形态 `llms.ToolCallResponse{ToolCallID, Content}` 塞进 user 消息的 Parts；go-llmx 用独立消息 `llmx.ToolResult(callID, result)`

## 第三步：非流式对话

```go
// langchaingo
resp, err := llm.GenerateContent(ctx, messages,
    llms.WithModel("deepseek-chat"),      // 请求级覆盖
    llms.WithTemperature(0.3),
    llms.WithMaxTokens(1024),
)
text := resp.Choices[0].Content

// go-llmx（方法同名，选项名基本一致）
resp, err := c.GenerateContent(ctx, messages,
    llmx.WithModel("deepseek-chat"),       // 请求级覆盖
    llmx.WithTemperature(0.3),
    llmx.WithMaxTokens(1024),
)
text, err := llmx.FirstText(resp)          // 空响应哨兵保护
```

**选项名映射**：

| langchaingo          | go-llmx                     | 备注            |
| -------------------- | --------------------------- | ------------- |
| `WithTemperature`    | `WithTemperature`           | 一致            |
| `WithMaxTokens`      | `WithMaxTokens`             | 一致            |
| `WithModel`          | `WithModel`                 | 一致（请求级覆盖）     |
| `WithStopWords`      | `WithStop`                  | 改名            |
| `WithCandidateCount` | `WithN`                     | 改名            |
| `WithStreamingFunc`  | `StreamGenerateContent` 参数  | **形态变更**，见第四步 |
| `WithTools`          | `WithTools` / `RunToolLoop` | 见第五步          |
| `WithJSONMode`\*     | `WithJSONMode`              | 一致            |
| `WithSeed`\*         | `WithSeed`                  | 一致            |

## 第四步：流式对话（形态变更，重点）

> langchaingo 的 `WithStreamingFunc` 回调签名是 `func(ctx, chunk []byte) error`。OpenAI 适配器传入的 chunk **多数时候是已提取的文本增量**，但存在三个坑：**语义不稳定**（工具调用帧时 chunk 变成聚合的 JSON 载荷而非文本）、**思考轨迹要另一个厂商特有 option**（`StreamingReasoningFunc`，且非全厂商支持）、**回调返回 error 即整体失败**（无"正常提前终止"语义，已收内容也拿不到）。
> go-llmx 把流式独立成 `StreamGenerateContent` 方法，回调收 `*llmx.Chunk` 结构化增量，四个字段各司其职，语义稳定。

### 4.1 基础流式打印

```go
// langchaingo
llm.GenerateContent(ctx, messages,
    llms.WithStreamingFunc(func(_ context.Context, chunk []byte) error {
        fmt.Print(string(chunk)) // 文本增量；⚠ 工具调用帧时是 JSON 载荷
        return nil               // ⚠ 返回 error = 整体失败，无法"正常终止"
    }),
)

// go-llmx：结构化 Chunk，文本/思考轨迹/工具增量/结束信号分字段，语义恒定
resp, err := c.StreamGenerateContent(ctx, messages, func(chunk *llmx.Chunk) error {
    fmt.Print(chunk.Content) // 文本增量（恒为纯文本，工具帧不会混入）
    return nil
})
// resp 与非流式同构：聚合全文 + 工具调用 + 用量 + 结束原因
```

### 4.2 收集全文（打字机效果之后还要整体结果）

```go
// langchaingo：手写累加回调（官方示例均此写法）
var sb strings.Builder
llm.GenerateContent(ctx, messages,
    llms.WithStreamingFunc(func(_ context.Context, chunk []byte) error {
        sb.Write(chunk)
        fmt.Print(string(chunk))
        return nil
    }),
)
full := sb.String()

// go-llmx ①：一行收集（StreamCollector 是内置 handler）
var full2 string
resp, err := c.StreamGenerateContent(ctx, messages, llmx.StreamCollector(&full2))

// go-llmx ②：聚合结果直接从 Response 取（无需自建 Builder）
text := resp.Choices[0].Text() // 流式聚合的完整文本，与非流式读取方式一致
```

### 4.3 思考轨迹（deepseek-reasoner / o1 类模型）

```go
// langchaingo：需要厂商特有 option，且回调签名因厂商而异（仅 OpenAI 适配器支持）
llm.GenerateContent(ctx, messages,
    llms.WithStreamingReasoningFunc( // ⚠ 非标准能力，换厂商可能没有
        func(_ context.Context, reasoning, content []byte) error {
            log.Printf("思考: %s", reasoning)
            return nil
        }),
)

// go-llmx：同一个 handler 的 Chunk.Reasoning 字段，所有适配器统一语义
c.StreamGenerateContent(ctx, messages, func(chunk *llmx.Chunk) error {
    if chunk.Reasoning != "" {
        log.Printf("思考: %s", chunk.Reasoning) // 思考轨迹增量
    }
    fmt.Print(chunk.Content)
    return nil
})
// 聚合结果同样可取：resp.Choices[0].Reasoning（完整思考轨迹）
```

### 4.4 提前终止（拿到首个结果即停，节省 token）

```go
// langchaingo：回调返回 error → 整个调用报错，且 GenerateContent 的
// 返回值为空——已收内容全部丢失，无法实现"够了就停"
llm.GenerateContent(ctx, messages,
    llms.WithStreamingFunc(func(_ context.Context, chunk []byte) error {
        if strings.Contains(string(chunk), "答案") {
            return errors.New("enough") // ⚠ 整体失败 + 内容丢失
        }
        fmt.Print(string(chunk))
        return nil
    }),
)

// go-llmx：ErrStopStream 哨兵——已收内容保留，正常语义收尾
resp, err := c.StreamGenerateContent(ctx, messages, func(chunk *llmx.Chunk) error {
    fmt.Print(chunk.Content)
    if strings.Contains(chunk.Content, "答案") {
        return llmx.ErrStopStream // 提前终止信号（非错误）
    }
    return nil
})
if errors.Is(err, llmx.ErrStreamClosed) { // 主动终止的标记哨兵
    // resp 依然可用：已收增量已聚合，text/usage 均保留
    log.Printf("提前终止，已收 %d 字", len(resp.Choices[0].Text()))
}
```

### 4.5 流式工具调用增量（观察模型正在组装的调用参数）

```go
// langchaingo：StreamingFunc 收到的 chunk 在工具帧时是 json.Marshal(delta)
// 的 ToolCall 数组，文本帧是纯文本字节——两种语义混在同一回调，需自行解析区分
llm.GenerateContent(ctx, messages, llms.WithTools(tools),
    llms.WithStreamingFunc(func(_ context.Context, chunk []byte) error {
        var calls []llms.ToolCall
        // 尝试按工具帧解析：首帧元素 Type=="function"，参数追加帧 Type 为空
        if err := json.Unmarshal(chunk, &calls); err == nil &&
            len(calls) > 0 && (calls[0].Type != "" || calls[0].FunctionCall != nil) {
            for _, c := range calls {
                if c.ID != "" {
                    argsByID[c.ID] = c.FunctionCall.Name // 首帧：新调用开始
                }
                argsBuf[c.ID] += c.FunctionCall.Arguments // 参数片段自行累积
            }
            return nil
        }
        fmt.Print(string(chunk)) // 文本帧
        return nil               // ⚠ 模型若输出合法 JSON 文本（如 "123"）会被误判为工具帧
    }),
)

// go-llmx：Chunk.ToolCallDelta 结构化区分（index 定位、arguments 逐段拼接）
c.StreamGenerateContent(ctx, messages, func(chunk *llmx.Chunk) error {
    if d := chunk.ToolCallDelta; d != nil {
        log.Printf("工具[%d] %s 参数追加: %s", d.Index, d.Name, d.Arguments)
    }
    return nil
})
// 若只想拿最终完整调用：RunToolLoop 内部已做聚合，直接读
// resp.Choices[0].ToolCalls()（Arguments 已拼好完整 JSON 串）
```

### 4.6 流式工具调用参数的完整处理范式（实战）

`ToolCallDelta` 的**语义约定**：`Arguments` 是**本帧片段**而非全量——模型把一次调用的参数 JSON 按帧拆开发送（首帧带 `Index/ID/Name`，后续帧仅带 `Arguments` 片段）。

**可运行完整示例**：[adapters/openai/examples/streamtools](./adapters/openai/examples/streamtools/main.go)（mock SSE 无需密钥，`go run ./examples/streamtools` 直接看双工具乱序交织 + 参数分段 + 两侧聚合对比）。

#### 范式一：实时 UI 反馈（打字机式展示参数组装过程）

```go
// 按 Index 归位追踪并行组装的多个工具调用
type toolProgress struct {
    id, name string
    args     strings.Builder
}

live := map[int]*toolProgress{}

resp, err := c.StreamGenerateContent(ctx, messages, func(chunk *llmx.Chunk) error {
    if d := chunk.ToolCallDelta; d != nil {
        p, ok := live[d.Index]
        if !ok {
            p = &toolProgress{id: d.ID, name: d.Name} // 首帧：ID/Name 携带
            live[d.Index] = p
            log.Printf("→ 模型开始调用工具: %s", d.Name)
        }
        p.args.WriteString(d.Arguments) // 逐段追加（本帧片段）
        fmt.Printf("\r%s 参数: %.60s", p.name, p.args.String())
    }
    return nil
})
```

#### 范式二：参数分段校验/流式解析（边收边处理）

```go
// 等参数收齐再 unmarshal 是常规做法；分段校验适合参数超大（如长文本生成类工具）
// 的提前失败场景：前缀已能判定不合法即提前终止，避免浪费后续 token
partialArgs := map[int]string{} // 已收参数前缀，按 Index 归位

c.StreamGenerateContent(ctx, messages, func(chunk *llmx.Chunk) error {
    d := chunk.ToolCallDelta
    if d == nil {
        return nil
    }
    if d.Name == "send_email" {
        // 首帧即知道工具名——高危工具可在参数生成期就介入审核
        log.Printf("高危工具调用开始: %s", d.Name)
    }
    partialArgs[d.Index] += d.Arguments
    // 对已开始的调用做前缀校验（如收件人黑名单）
    if strings.Contains(partialArgs[d.Index], "spam-target@x.com") {
        return llmx.ErrStopStream // 命中即停，已收内容仍保留
    }
    return nil
})
```

#### 收尾对比：增量 vs 最终聚合

```go
// 流式回调期间：d.Arguments 为片段（`{"q":` → `"go"}`），需要自己拼
// 流式结束后：聚合器已拼好，直接从 Response 取完整调用——两者并存，按需取用
resp, err := c.StreamGenerateContent(ctx, messages, handler)
if err == nil {
    for _, call := range resp.Choices[0].ToolCalls() {
        var args map[string]any
        _ = json.Unmarshal([]byte(call.Arguments), &args) // 完整 JSON 串，可直接反序列化
        log.Printf("最终调用 %s(%s)", call.Name, call.Arguments)
    }
}
```

**关键语义提醒**（迁移时最容易踩的点）：

1. **首帧字段**：`ID/Name` 仅在首帧携带（后续帧为空串），按 `Index` 追踪同一调用，**不要按 Name 判帧**
2. **Arguments 是片段**：`d.Arguments` ≠ 完整参数；单帧语义是"追加"，全量在流结束后的 `resp.Choices[0].ToolCalls()`
3. **乱序防御**：网关可能乱序/非 0 起 index，go-llmx 聚合器内部按 `maxIndex` 收口（业务侧无需处理，但自定义拼接时建议同样按 Index 归位而非 append 顺序）
4. **透传语义全适配器一致**：OpenAI 的 `tool_calls[].function.arguments`、Anthropic 的 `input_json_delta.partial_json`、Ollama 的整帧工具调用，统一归一为 `ToolCallDelta{Index, ID, Name, Arguments}`——迁移业务代码时无需按厂商分叉

**差异总结**：

| 关注点  | langchaingo                        | go-llmx                                                         |
| ---- | ---------------------------------- | --------------------------------------------------------------- |
| 回调载荷 | `[]byte`，语义不稳定（文本/工具 JSON 混装）      | `*llmx.Chunk` 四字段（Content/Reasoning/ToolCallDelta/FinishReason） |
| 思考轨迹 | `WithStreamingReasoningFunc`（厂商特有） | 同一 handler 的 `Chunk.Reasoning`（全适配器统一）                          |
| 提前终止 | 返回 error 即失败，已收内容丢失                | `ErrStopStream` 哨兵，已收内容随 Response 保留                            |
| 结束信号 | 需另行从返回的 response 判断                | 结束帧 `Chunk{FinishReason, Usage}` 显式携带                           |
| 聚合全文 | 手写 Builder 累加                      | `StreamCollector` 或直接读 `resp.Choices[0].Text()`                 |
| 流式开关 | `WithStreamingFunc` 传不传（选项形态）      | 调 `StreamGenerateContent` 还是 `GenerateContent`（方法形态）            |

## 第五步：工具调用（最大简化点）

```go
// langchaingo：Tool 定义嵌套，执行循环需自写或引入 agents
import "github.com/tmc/langchaingo/llms"

tools := []llms.Tool{{
    Type: "function",
    Function: &llms.FunctionDefinition{
        Name:        "weather",
        Description: "查询城市天气",
        Parameters:  map[string]any{...}, // 手写 JSON Schema
    },
}}
// ① 手动循环：GenerateContent(WithTools) → 解析 ToolCalls → 执行 → 组装 ToolChatMessage → 再调用...
// ② 或 agents.Initialize MRKL 脚手架（Thought/Action/Observation 字符串协议）

// go-llmx：定义与执行器合一，RunToolLoop 一行驱动
import "github.com/kamalyes/go-llmx/tool"

type weatherArgs struct {
    City string `json:"city"`
}

resp, err := tool.RunToolLoop(ctx, c, messages, []tool.Tool{{
    Name:        "weather",
    Description: "查询城市天气",
    Parameters:  tool.SchemaOf(weatherArgs{}), // Go 结构体反射生成 Schema
    Func: func(ctx context.Context, arguments string) (string, error) {
        return fetchWeather(arguments) // 执行器内联
    },
}})
```

**差异说明**：

- `FunctionDefinition` 嵌套 → `Tool` 扁平结构，**定义与** **`Func`** **执行器绑定**（lookup 免维护）
- Schema 手写 → `SchemaOf(结构体)` 反射生成（`omitempty` → 可选字段，json tag → 属性名）
- 手动循环 → `RunToolLoop`：模型请求 → 执行 → 结果回传 → 循环直至最终回答；迭代上限默认 8（`WithMaxToolIterations` 调整）；工具执行失败不中断，错误信息回传给模型自主调整
- MRKL 字符串协议 → 原生 function calling 协议（准确性与调试性更好）

## 第六步：chains

```go
// langchaingo：map 进出 + options 变参
import (
    "github.com/tmc/langchaingo/chains"
    "github.com/tmc/langchaingo/prompts"
)

llmChain := chains.NewLLMChain(llm, prompts.NewPromptTemplate(
    "给 {{.input}} 起一个产品名", []string{"input"},
))
out, err := chains.Call(ctx, llmChain, map[string]any{"input": "AI 代码助手"})
name := out["text"].(string)

// go-llmx：结构体字段装配，string 进出
import (
    "github.com/kamalyes/go-llmx/chain"
    "github.com/kamalyes/go-llmx/prompt"
)

tpl, _ := prompt.New("给 {{.Input}} 起一个产品名")
llmChain := &chain.LLMChain{Model: c, Prompt: tpl}
name, err := llmChain.Run(ctx, "AI 代码助手")
```

**会话链**：

```go
// langchaingo（NewConversation 返回 LLMChain 单值，无 error）
import (
    "github.com/tmc/langchaingo/chains"
    "github.com/tmc/langchaingo/memory"
)

conv := chains.NewConversation(llm, memory.NewConversationBuffer())
out, err := chains.Call(ctx, conv, map[string]any{"input": "我叫小明"})
answer := out["text"].(string)

// go-llmx（结构体字面量 + 单值进出）
import (
    "github.com/kamalyes/go-llmx/chain"
    "github.com/kamalyes/go-llmx/memory"
)

conv := &chain.ConversationChain{
    Model:  c,
    Memory: memory.NewWindow(10), // 滑窗；NewBuffer() 为全量
}
answer, err := conv.Run(ctx, "我叫小明")
```

**差异说明**：

- `chains.Call(map[string]any → map[string]any)` → `Run(string → string)`：多值编排请用 `SequentialChain` 或直接组合
- 构造从函数 + options → 结构体字面量（依赖显式可见）
- 会话链都是"内置对话模板 + 记忆回写"语义，go-llmx 不预设英文模板（自带 Prompt 可定制中文模板）
- 模板变量注入 `chainInput{Input}` 内置包装（变量名 `.Input`）

## 第七步：embeddings

```go
// langchaingo：NewEmbedder 包装实现 EmbedderClient 的 LLM（OpenAI 客户端自带 CreateEmbedding）
import "github.com/tmc/langchaingo/embeddings"

e, err := embeddings.NewEmbedder(llm, embeddings.WithBatchSize(64))
vecs, err := e.EmbedDocuments(ctx, []string{"a", "b"})
q, err    := e.EmbedQuery(ctx, "查询")

// go-llmx（独立子包，与对话客户端解耦配置）
import lcembed "github.com/kamalyes/go-llmx/embeddings/openai"

e := lcembed.New(os.Getenv("OPENAI_API_KEY"))
vecs, err := e.EmbedDocuments(ctx, []string{"a", "b"})
q, err    = e.EmbedQuery(ctx, "查询")
```

**差异说明**：

- 接口同名同形（`EmbedDocuments/EmbedQuery`）
- 嵌入能力挂载：langchaingo 的 `NewEmbedder(llm)` **复用对话客户端**（llm 须实现 `EmbedderClient.CreateEmbedding`）；go-llmx 嵌入是**独立构造**（嵌入与对话可指向不同厂商/网关，独立热更）
- langchaingo 的 `NewEmbedder` 返回 error（含批处理参数校验）；go-llmx 的 `New` 无 error
- 批处理：langchaingo 由 `EmbedderImpl` 层做 `WithBatchSize` 切批；go-llmx 适配器内部按厂商最优批策略直发

## 第八步：vectorstores（显式两步）

```go
// langchaingo：embedder 通过 WithEmbedder option 注入 store，检索传查询字符串
import "github.com/tmc/langchaingo/vectorstores/chroma"

store, err := chroma.New(
    chroma.WithChromaURL("http://localhost:8000"),
    chroma.WithEmbedder(e), // 向量化能力注入
)
docs, err := store.SimilaritySearch(ctx, "退款政策", 4, // topK 是位置参数
    vectorstores.WithScoreThreshold(0.7), // 其余走 option
)

// go-llmx：向量化与检索解耦，显式两步（契约不含 embedder 依赖）
q, err   := e.EmbedQuery(ctx, "退款政策")
docs, err := store.SimilaritySearch(ctx, q, 4, llmx.Filter{"source": "doc-1"})
```

**差异说明**：

- 检索入参：langchaingo 传 `query string`（store 内部用注入的 embedder 向量化）→ go-llmx **显式传入向量**（可换嵌入厂商不换存储，store 零嵌入依赖）
- topK：langchaingo 是位置参数 `numDocuments` + `WithScoreThreshold` 等 option → go-llmx 同为位置参数 + 变参 `Filter`
- 过滤：`vectorstores.WithFilters(any)`（各实现自行解析结构）→ 变参 `llmx.Filter`（单 Filter 多 KV 为 AND，语义收窄为等值匹配）

## 第九步：错误处理

```go
// langchaingo：标准化错误体系（llms.Error 携带 ErrorCode，支持 errors.Is 与判定函数）
switch {
case errors.Is(err, llms.ErrRateLimit):            // 哨兵：*Error{Code: rate_limit}
case llms.IsAuthenticationError(err):             // 判定函数：errors.As + Code 比对
case llms.IsQuotaExceededError(err):               // 独立错误码：配额耗尽
case llms.IsTimeoutError(err):                     // 超时（兼容 context.DeadlineExceeded）
}

// go-llmx：纯哨兵判定（12 个哨兵错误，errors.Is 一条路）
switch {
case errors.Is(err, llmx.ErrRateLimited):          // 429/配额耗尽 → 退避重试
case errors.Is(err, llmx.ErrUnauthorized):         // 401/403 → 换 key
case errors.Is(err, llmx.ErrProviderUnavailable):  // 网络不可达/404 → 灾备切换
case errors.Is(err, llmx.ErrAPIServerError):       // 5xx → 重试或降级
case errors.Is(err, llmx.ErrEmptyResponse):        // 空响应 → 兜底文案
}
```

**差异说明**：

- 两边都是**结构化错误**（非错误串匹配），迁移是把 `llms.ErrXxx` 哨兵/判定函数换成 `llmx.ErrXxx` 哨兵
- 形态：langchaingo 的 `*llms.Error{Code, Message, Provider, Details, Cause}`（错误码枚举 + 判定函数双轨）→ go-llmx 的纯哨兵错误（`errors.Is` 单轨，wrap 链保留上下文）
- 粒度差异：langchaingo 的 `quota_exceeded`/`content_filter`/`token_limit` 是独立错误码；go-llmx 将配额归入 `ErrRateLimited`、内容审核归入 `ErrInvalidRequest`（12 哨兵收敛，业务分支更少；需要细分场景可解 wrap 链上的 `*adapter.ErrorBody`）

## 无对应清单

| langchaingo 能力                  | go-llmx 替代方案                           | 说明                                          |
| ------------------------------- | -------------------------------------- | ------------------------------------------- |
| `agents` MRKL/ReAct 脚手架         | `tool.RunToolLoop`                     | 原生 function calling 替代 Thought/Action 字符串协议 |
| 六层 callbacks 体系                 | `StreamHandler` 单回调                    | 需要横切观测时在 handler 内埋点，或包装 `llmx.Model`       |
| document loaders（30+ 格式）        | 自行读取 + `textsplitter.SplitDocuments`  | 载入是 IO 问题，不进 LLM 库                          |
| output parsers                  | `llmx.WithJSONMode` + `json.Unmarshal` | 结构化输出靠协议而非解析器                               |
| `memory.NewConversationTokenBuffer` | `memory.NewWindow(k)`                  | token 计数需 tiktoken（引入重依赖），按轮次滑窗替代           |
| 30+ 厂商适配器                       | 3 个协议栈                                 | OpenAI 兼容协议覆盖绝大多数主流厂商；其余按 `adapter` 基座扩展    |
| `schema.Document.Score` 相似度回传   | 检索结果不含 score                           | 如需 score 可在 store 实现层扩展返回                   |
| testcontainers 集成测试             | `FakeModel` + httptest                 | 无网络依赖、无 Docker、毫秒级                          |

## 迁移检查清单

- [ ] 确认使用的 langchaingo 子包在对照表中（不在则看[无对应清单](#无对应清单)）
- [ ] 客户端构造改为显式密钥首参 + With 选项
- [ ] 消息构造：`llms.TextParts(role, s...)` 改为 `llmx.User/System/Assistant` 函数（多模态 `llms.TextPart/ImageURLPart` 改 `llmx.TextPart/ImagePart`）
- [ ] `resp.Choices[0].Content` 读取改为 `llmx.FirstText(resp)`
- [ ] `WithStreamingFunc` 回调改为 `StreamGenerateContent` handler（享受结构化 Chunk）
- [ ] 工具调用改为 `Tool` + `RunToolLoop`（Schema 用 `SchemaOf` 生成）
- [ ] `chains.Call(map)` 改为 `chain.Run(string)` 或 `SequentialChain`；`chains.NewConversation` 改为 `chain.ConversationChain` 字面量
- [ ] `embeddings.NewEmbedder(llm)` 改为各厂商 embedder 子包 `New(apiKey)`
- [ ] 检索改为 EmbedQuery → SimilaritySearch 两步（store 构造去掉 `WithEmbedder`）
- [ ] 错误分支 `llms.ErrXxx` 哨兵/`llms.IsXxxError` 判定函数改为 `llmx.ErrXxx` 哨兵 `errors.Is`
- [ ] 测试桩从 mock LLM 改为 `llmx.NewFakeModel("预设回复")`

> 迁移遇阻的形态请对照 [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) 了解内部实现原理后再评估扩展点。

