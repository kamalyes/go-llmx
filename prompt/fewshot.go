/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-07-16 15:02:33
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-07-16 15:02:33
 * @FilePath: \go-llmx\prompt\fewshot.go
 * @Description: 少样本提示词 —— 示例选择器（长度预算 / 语义 top-K）+
 * FewShot 模板，对齐 langchaingo few_shot 能力且复用标准库模板引擎
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package prompt

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	llmx "github.com/kamalyes/go-llmx"
)

// fewShotSep 示例块之间默认分隔.
// [EN] Default separator between example blocks.
const fewShotSep = "\n\n"

// fewShotInputKey 语义比对的默认示例字段.
// [EN] Default example field for semantic comparison.
const fewShotInputKey = "Input"

// Example 少样本示例（键值即示例模板变量，如 Input/Output）.
// [EN] A few-shot example (keys feed the example template).
type Example map[string]string

// ExampleSelector 示例选择器：为一次调用从全量示例中挑选子集
// （控制 prompt 长度 / 只给相关示例）.
// [EN] Example selector: pick a subset per call.
type ExampleSelector interface {
	// Select 按输入挑选相关示例（保持稳定序）.
	// [EN] Select relevant examples for the input (stable order).
	Select(ctx context.Context, input string, examples []Example) []Example
}

// LengthSelector 长度预算选择器：按近似 token 预算先入先选，
// 超预算即停（不回溯贪心，简单可预期）.
// [EN] Length-budget selector: first-fit until the budget.
type LengthSelector struct {
	// maxTokens 近似 token 预算（rune/4，与 memory.ApproxTokens 同一规则）.
	// [EN] Approximate token budget (rune/4).
	maxTokens int
}

// NewLengthSelector 构造长度选择器（maxTokens <= 0 时不限制）.
// [EN] Build a length selector (unbounded when non-positive).
func NewLengthSelector(maxTokens int) *LengthSelector {
	return &LengthSelector{maxTokens: maxTokens}
}

// Select 实现 ExampleSelector（先入先选至预算耗尽；至少保留一个示例）.
// [EN] Implement ExampleSelector (first-fit; keeps at least one).
func (s *LengthSelector) Select(_ context.Context, _ string, examples []Example) []Example {
	if s.maxTokens <= 0 || len(examples) == 0 {
		return examples
	}
	budget := s.maxTokens * 4
	var out []Example
	for _, e := range examples {
		n := exampleRunes(e)
		if budget-n < 0 && len(out) > 0 {
			break
		}
		out = append(out, e)
		budget -= n
	}
	return out
}

// exampleRunes 示例全部值的字符量（预算估算）.
// [EN] Total runes of an example's values.
func exampleRunes(e Example) int {
	n := 0
	for _, v := range e {
		n += len([]rune(v))
	}
	return n
}

// SemanticSelector 语义选择器：embedder 向量相似度 top-K
// （查询与示例 Input 字段比对；无输入时全量返回）.
// [EN] Semantic selector: embedder cosine top-K.
type SemanticSelector struct {
	// embedder 向量嵌入器（任意 llmx.Embedder 实现）.
	// [EN] Any llmx.Embedder implementation.
	embedder llmx.Embedder

	// topK 选取数量（<= 0 时取全部）.
	// [EN] Count to keep.
	topK int

	// inputKey 示例中用于比对的字段（默认 "Input"）.
	// [EN] Example field compared against the query.
	inputKey string
}

// NewSemanticSelector 构造语义选择器.
// [EN] Build a semantic selector.
func NewSemanticSelector(embedder llmx.Embedder, topK int, opts ...SemanticOption) *SemanticSelector {
	s := &SemanticSelector{
		embedder: embedder,
		topK:     topK,
		inputKey: fewShotInputKey,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// SemanticOption 语义选择器配置.
// [EN] Semantic selector option.
type SemanticOption func(*SemanticSelector)

// WithInputKey 配置示例比对字段名.
// [EN] Configure the example field compared.
func WithInputKey(key string) SemanticOption {
	return func(s *SemanticSelector) { s.inputKey = key }
}

// Select 实现 ExampleSelector：查询向量与示例向量余弦比对取 top-K
// （正分入候选，降序稳定排序；示例向量每批一次性嵌入）.
// [EN] Implement ExampleSelector (cosine top-K, batched).
func (s *SemanticSelector) Select(ctx context.Context, input string, examples []Example) []Example {
	if strings.TrimSpace(input) == "" || len(examples) == 0 {
		return examples
	}
	if s.topK > 0 && len(examples) <= s.topK {
		return examples
	}

	qv, err := s.embedder.EmbedQuery(ctx, input)
	if err != nil {
		return examples // 嵌入失败降级全量，不阻断渲染
	}

	texts := make([]string, len(examples))
	for i, e := range examples {
		texts[i] = e[s.inputKey]
	}
	vecs, err := s.embedder.EmbedDocuments(ctx, texts)
	if err != nil || len(vecs) != len(examples) {
		return examples // 同上：降级而非失败
	}

	scores := make([]float64, len(examples))
	for i, v := range vecs {
		scores[i] = cosineSimilarity(qv, v)
	}
	idx := make([]int, len(examples))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return scores[idx[a]] > scores[idx[b]] })
	if s.topK > 0 && len(idx) > s.topK {
		idx = idx[:s.topK]
	}
	out := make([]Example, 0, len(idx))
	for _, i := range idx {
		out = append(out, examples[i])
	}
	return out
}

// cosineSimilarity 余弦相似度（维度不一致或零向量返回 0）.
// [EN] Cosine similarity (0 on mismatch or zero vectors).
func cosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// FewShot 少样本模板：前缀 +（示例模板渲染的选中示例）+ 后缀.
// [EN] Few-shot template: prefix + rendered examples + suffix.
type FewShot struct {
	// exampleTpl 单个示例的渲染模板（变量来自 Example 键）.
	// [EN] Per-example template (fed by Example keys).
	exampleTpl *Template

	// examples 全量示例池.
	// [EN] Full example pool.
	examples []Example

	// selector 选择器（nil 全量注入）.
	// [EN] Selector (nil injects all).
	selector ExampleSelector

	// prefix 示例块前文本.
	// [EN] Text before examples.
	prefix string

	// suffix 示例块后文本.
	// [EN] Text after examples.
	suffix string

	// sep 示例块分隔符.
	// [EN] Separator between examples.
	sep string
}

// NewFewShot 构造少样本模板（exampleTpl 的变量来自示例键）.
// [EN] Build a few-shot template.
func NewFewShot(exampleTpl *Template, examples ...Example) *FewShot {
	return &FewShot{
		exampleTpl: exampleTpl,
		examples:   examples,
		sep:        fewShotSep,
	}
}

// WithSelector 注入示例选择器（nil 清除）.
// [EN] Inject an example selector.
func (f *FewShot) WithSelector(sel ExampleSelector) *FewShot {
	f.selector = sel
	return f
}

// WithPrefix 设置前缀文本.
// [EN] Set the prefix.
func (f *FewShot) WithPrefix(prefix string) *FewShot {
	f.prefix = prefix
	return f
}

// WithSuffix 设置后缀文本.
// [EN] Set the suffix.
func (f *FewShot) WithSuffix(suffix string) *FewShot {
	f.suffix = suffix
	return f
}

// WithSeparator 覆盖示例分隔符（默认空行）.
// [EN] Override the separator.
func (f *FewShot) WithSeparator(sep string) *FewShot {
	f.sep = sep
	return f
}

// Render 渲染少样本提示词：选中示例逐个经 exampleTpl 渲染后拼接，
// 前后缀包裹；input 供语义选择器比对（无选择器时传空即可）.
// [EN] Render: selected examples rendered and joined, wrapped
// by prefix/suffix; input feeds semantic selectors.
func (f *FewShot) Render(ctx context.Context, input string) (string, error) {
	if f.exampleTpl == nil {
		return "", fmt.Errorf("%w: few-shot requires an example template", llmx.ErrInvalidRequest)
	}

	chosen := f.examples
	if f.selector != nil {
		chosen = f.selector.Select(ctx, input, f.examples)
	}

	blocks := make([]string, 0, len(chosen))
	for _, e := range chosen {
		out, err := f.exampleTpl.Render(e)
		if err != nil {
			return "", err
		}
		blocks = append(blocks, out)
	}

	var b strings.Builder
	if f.prefix != "" {
		b.WriteString(f.prefix)
		b.WriteString(f.sep)
	}
	b.WriteString(strings.Join(blocks, f.sep))
	if f.suffix != "" {
		b.WriteString(f.sep)
		b.WriteString(f.suffix)
	}
	return b.String(), nil
}
