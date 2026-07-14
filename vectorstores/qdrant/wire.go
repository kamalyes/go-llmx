/*
 * @Author: wmxuan 836551135@qq.com
 * @Date: 2026-07-14 21:33:08
 * @LastEditors: wmxuan 836551135@qq.com
 * @LastEditTime: 2026-07-14 21:38:26
 * @FilePath: \go-llmx\vectorstores\qdrant\wire.go
 * @Description: Qdrant REST 协议层 —— 集合创建/信息请求构造、批量写入
 * batch 编解码、检索过滤（metadata 嵌套 key 匹配）、命中行集还原
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package lcqdrant

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"

	llmx "github.com/kamalyes/go-llmx"
	transport "github.com/kamalyes/go-llmx/transport"
)

// isNotFound 判定 404（集合不存在，触发建链路）.
// [EN] Detect 404 (collection absent; triggers creation).
func isNotFound(err error) bool {
	se, ok := err.(*transport.StatusError)
	return ok && se.Status == 404
}

// wireCreateRequest 建集合请求（unnamed 向量，langchaingo 默认同构）.
// [EN] Create-collection request (unnamed vector, langchaingo default).
type wireCreateRequest struct {
	Vectors *wireVectorsConfig `json:"vectors"`
}

// wireVectorsConfig 默认向量参数（size + distance）.
// [EN] Default vector params (size + distance).
type wireVectorsConfig struct {
	Size     int    `json:"size"`
	Distance string `json:"distance"`
}

// buildCreateRequest 构造建集合请求.
// [EN] Build the create-collection request.
func buildCreateRequest(dim int, distance string) *wireCreateRequest {
	return &wireCreateRequest{
		Vectors: &wireVectorsConfig{Size: dim, Distance: distance},
	}
}

// wireCollectionInfo 集合信息响应（存量集合维度识别）.
// [EN] Collection-info response (dimension detection for existing collections).
type wireCollectionInfo struct {
	Result struct {
		Config struct {
			Params struct {
				Vectors *wireVectorsConfig `json:"vectors"`
			} `json:"params"`
		} `json:"config"`
	} `json:"result"`
}

// wireBatch 批量写入载荷（ids/vectors/payloads 三列对齐）.
// [EN] Batch upsert payload (ids/vectors/payloads aligned).
type wireBatch struct {
	IDs      []string         `json:"ids"`
	Vectors  [][]float64      `json:"vectors"`
	Payloads []map[string]any `json:"payloads"`
}

// wireUpsertRequest 写入请求.
// [EN] Upsert request.
type wireUpsertRequest struct {
	Batch *wireBatch `json:"batch"`
}

// buildUpsertRequest 构造批量写入请求（UUIDv4 主键 + payload 两字段）.
// [EN] Build the batch upsert request (UUIDv4 keys + two-field payloads).
func buildUpsertRequest(docs []llmx.Document, vectors [][]float64) *wireUpsertRequest {
	b := &wireBatch{
		IDs:      make([]string, len(docs)),
		Vectors:  vectors,
		Payloads: make([]map[string]any, len(docs)),
	}
	for i, d := range docs {
		b.IDs[i] = newUUID()
		row := map[string]any{payloadContent: d.PageContent}
		if d.Metadata != nil {
			row[payloadMetadata] = d.Metadata
		}
		b.Payloads[i] = row
	}
	return &wireUpsertRequest{Batch: b}
}

// wireResultResponse 通用结果响应（result 兼容 bool 与对象两种形态）.
// [EN] Common result response (result as bool or object).
type wireResultResponse struct {
	Result any `json:"result"`
}

// ok 判定操作成功（bool 真 / 对象 status completed）.
// [EN] Decide success (true bool / object status completed).
func (w wireResultResponse) ok() bool {
	switch v := w.Result.(type) {
	case bool:
		return v
	case map[string]any:
		return v["status"] == "completed"
	default:
		return false
	}
}

// wireMatch 匹配条件（value 支持字符串/数字/布尔）.
// [EN] Match condition (value as string/number/bool).
type wireMatch struct {
	Value any `json:"value"`
}

// wireCondition 单条件.
// [EN] Single condition.
type wireCondition struct {
	Key   string     `json:"key"`
	Match *wireMatch `json:"match"`
}

// wireFilter 过滤器（must AND 语义）.
// [EN] Filter (must = AND).
type wireFilter struct {
	Must []*wireCondition `json:"must,omitempty"`
}

// wireSearchRequest 检索请求.
// [EN] Search request.
type wireSearchRequest struct {
	Vector      []float64  `json:"vector"`
	Limit       int        `json:"limit"`
	WithPayload bool       `json:"with_payload"`
	Filter      *wireFilter `json:"filter,omitempty"`
}

// buildSearchRequest 构造检索请求.
// [EN] Build the search request.
func buildSearchRequest(query []float64, topK int, filters []llmx.Filter) *wireSearchRequest {
	if topK <= 0 {
		topK = defaultTopK
	}
	return &wireSearchRequest{
		Vector:      query,
		Limit:       topK,
		WithPayload: true,
		Filter:      buildFilter(filters),
	}
}

// buildFilter 构造过滤（metadata.<k> 嵌套 key 匹配，键排序确定性）.
// [EN] Build the filter (metadata.<k> nested-key match, key-sorted).
func buildFilter(filters []llmx.Filter) *wireFilter {
	if len(filters) == 0 {
		return nil
	}
	type kv struct {
		k string
		v any
	}
	parts := make([]kv, 0, len(filters))
	for _, f := range filters {
		for k, v := range f {
			parts = append(parts, kv{payloadMetadata + "." + k, v})
		}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].k < parts[j].k })
	must := make([]*wireCondition, 0, len(parts))
	for _, p := range parts {
		must = append(must, &wireCondition{Key: p.k, Match: &wireMatch{Value: p.v}})
	}
	return &wireFilter{Must: must}
}

// wireSearchResult 检索响应（命中含 score 与 payload）.
// [EN] Search response (hits carry score and payload).
type wireSearchResult struct {
	Result []struct {
		ID      string         `json:"id"`
		Score   float64        `json:"score"`
		Payload map[string]any `json:"payload"`
	} `json:"result"`
}

// hitsToDocuments 命中还原为文档（score 丢弃，payload 两字段还原）.
// [EN] Restore hits to documents (score dropped; two payload fields).
func hitsToDocuments(hits []struct {
	ID      string         `json:"id"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}) []llmx.Document {
	if len(hits) == 0 {
		return nil
	}
	docs := make([]llmx.Document, 0, len(hits))
	for _, h := range hits {
		doc := llmx.Document{PageContent: asString(h.Payload[payloadContent])}
		switch meta := h.Payload[payloadMetadata].(type) {
		case map[string]any:
			doc.Metadata = meta
		}
		docs = append(docs, doc)
	}
	return docs
}

// asString 宽松字符串还原.
// [EN] Lenient string restoration.
func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

// newUUID 生成 UUIDv4 文本（crypto/rand，免第三方依赖）.
// [EN] Generate a UUIDv4 string (crypto/rand; no third-party deps).
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	dst := make([]byte, 36)
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst)
}
