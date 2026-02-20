package core

import (
	"encoding/json"
	"strings"
)

const (
	stateNormal       = 0
	stateTagBuffering = 1
	stateInsideCall   = 2
	stateCloseTag     = 3
)

const openTag = "<tool_call>"
const closeTag = "</tool_call>"

// ParsedToolCall 表示解析出的一次工具调用
type ParsedToolCall struct {
	Name      string `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// toolCallParser 增量解析流式文本中的 <tool_call>...</tool_call> 标签
type toolCallParser struct {
	state         int
	tagBuffer     strings.Builder // 缓冲可能的标签文本
	callBuffer    strings.Builder // 缓冲 <tool_call> 内的 JSON
	closeBuffer   strings.Builder // 缓冲可能的 </tool_call> 标签
	foundToolCall bool
}

func newToolCallParser() *toolCallParser {
	return &toolCallParser{state: stateNormal}
}

// Feed 输入一段文本，返回应作为普通内容输出的文本和已完成的工具调用
func (p *toolCallParser) Feed(text string) (normalText string, toolCalls []ParsedToolCall) {
	var normal strings.Builder

	for i := 0; i < len(text); i++ {
		ch := text[i]

		switch p.state {
		case stateNormal:
			if ch == '<' {
				p.tagBuffer.Reset()
				p.tagBuffer.WriteByte(ch)
				p.state = stateTagBuffering
			} else {
				normal.WriteByte(ch)
			}

		case stateTagBuffering:
			p.tagBuffer.WriteByte(ch)
			buf := p.tagBuffer.String()
			if buf == openTag {
				// 匹配到 <tool_call>，进入内容收集状态
				p.callBuffer.Reset()
				p.state = stateInsideCall
			} else if strings.HasPrefix(openTag, buf) {
				// 还可能匹配，继续缓冲
			} else {
				// 不匹配，把缓冲区内容当作普通文本输出
				normal.WriteString(buf)
				p.state = stateNormal
			}

		case stateInsideCall:
			if ch == '<' {
				p.closeBuffer.Reset()
				p.closeBuffer.WriteByte(ch)
				p.state = stateCloseTag
			} else {
				p.callBuffer.WriteByte(ch)
			}

		case stateCloseTag:
			p.closeBuffer.WriteByte(ch)
			buf := p.closeBuffer.String()
			if buf == closeTag {
				// 匹配到 </tool_call>，解析工具调用
				tc := p.parseToolCall(p.callBuffer.String())
				if tc != nil {
					toolCalls = append(toolCalls, *tc)
					p.foundToolCall = true
				} else {
					// 解析失败，作为普通文本输出
					normal.WriteString(openTag)
					normal.WriteString(p.callBuffer.String())
					normal.WriteString(closeTag)
				}
				p.callBuffer.Reset()
				p.state = stateNormal
			} else if strings.HasPrefix(closeTag, buf) {
				// 还可能匹配，继续缓冲
			} else {
				// 不匹配 </tool_call>，把缓冲区内容放回 callBuffer
				p.callBuffer.WriteString(buf)
				p.state = stateInsideCall
			}
		}
	}

	return normal.String(), toolCalls
}

// Flush 在流结束时调用，刷出所有缓冲区
func (p *toolCallParser) Flush() (normalText string, toolCalls []ParsedToolCall) {
	var normal strings.Builder

	switch p.state {
	case stateTagBuffering:
		// 未完成的标签，作为普通文本
		normal.WriteString(p.tagBuffer.String())
	case stateInsideCall:
		// 未关闭的 <tool_call>，尝试解析
		tc := p.parseToolCall(p.callBuffer.String())
		if tc != nil {
			toolCalls = append(toolCalls, *tc)
			p.foundToolCall = true
		} else {
			normal.WriteString(openTag)
			normal.WriteString(p.callBuffer.String())
		}
	case stateCloseTag:
		// 未完成的关闭标签，尝试解析已有内容
		tc := p.parseToolCall(p.callBuffer.String())
		if tc != nil {
			toolCalls = append(toolCalls, *tc)
			p.foundToolCall = true
		} else {
			normal.WriteString(openTag)
			normal.WriteString(p.callBuffer.String())
			normal.WriteString(p.closeBuffer.String())
		}
	}

	p.state = stateNormal
	p.tagBuffer.Reset()
	p.callBuffer.Reset()
	p.closeBuffer.Reset()

	return normal.String(), toolCalls
}

// HasToolCalls 返回是否找到了至少一个工具调用
func (p *toolCallParser) HasToolCalls() bool {
	return p.foundToolCall
}

// IsBuffering 返回解析器是否正在缓冲（不应输出 content）
func (p *toolCallParser) IsBuffering() bool {
	return p.state != stateNormal
}

func (p *toolCallParser) parseToolCall(raw string) *ParsedToolCall {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var tc ParsedToolCall
	if err := json.Unmarshal([]byte(raw), &tc); err != nil {
		return nil
	}
	if tc.Name == "" {
		return nil
	}
	// 如果 arguments 为空，设为 "{}"
	if len(tc.Arguments) == 0 {
		tc.Arguments = json.RawMessage("{}")
	}
	return &tc
}
