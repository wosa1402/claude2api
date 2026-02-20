// utils/chat_utils.go
package utils

import (
	"claude2api/config"
	"claude2api/logger"
	"encoding/json"
	"fmt"
	"strings"
)

// ChatRequestProcessor handles common chat request processing logic
type ChatRequestProcessor struct {
	Prompt      strings.Builder
	RootPrompt  strings.Builder
	ImgDataList []string
	HasTools    bool
}

// NewChatRequestProcessor creates a new processor instance
func NewChatRequestProcessor() *ChatRequestProcessor {
	return &ChatRequestProcessor{
		Prompt:      strings.Builder{},
		RootPrompt:  strings.Builder{},
		ImgDataList: []string{},
	}
}

// ProcessMessages processes the messages array into a prompt and extracts images
func (p *ChatRequestProcessor) ProcessMessages(messages []map[string]interface{}) {
	if config.ConfigInstance.PromptDisableArtifacts {
		p.Prompt.WriteString("System: Forbidden to use <antArtifac> </antArtifac> to wrap code blocks, use markdown syntax instead, which means wrapping code blocks with ``` ```\n\n")
	}

	for _, msg := range messages {
		role, roleOk := msg["role"].(string)
		if !roleOk {
			continue // Skip invalid format
		}

		// 处理 tool 角色消息（工具执行结果）
		if role == "tool" {
			toolCallID, _ := msg["tool_call_id"].(string)
			content, _ := msg["content"].(string)
			p.Prompt.WriteString(fmt.Sprintf("<tool_result tool_call_id=\"%s\">\n%s\n</tool_result>\n\n", toolCallID, content))
			continue
		}

		// 处理 assistant 消息中的 tool_calls 历史
		if role == "assistant" {
			if toolCalls, ok := msg["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
				p.Prompt.WriteString(GetRolePrefix(role))
				// 输出文本内容（如果有）
				if contentStr, ok := msg["content"].(string); ok && contentStr != "" {
					p.Prompt.WriteString(contentStr + "\n")
				}
				// 输出工具调用
				for _, tc := range toolCalls {
					tcMap, ok := tc.(map[string]interface{})
					if !ok {
						continue
					}
					fn, ok := tcMap["function"].(map[string]interface{})
					if !ok {
						continue
					}
					name, _ := fn["name"].(string)
					args, _ := fn["arguments"].(string)
					p.Prompt.WriteString(fmt.Sprintf("<tool_call>\n{\"name\": \"%s\", \"arguments\": %s}\n</tool_call>\n", name, args))
				}
				p.Prompt.WriteString("\n")
				continue
			}
		}

		content, exists := msg["content"]
		if !exists {
			continue
		}

		p.Prompt.WriteString(GetRolePrefix(role))

		switch v := content.(type) {
		case string: // If content is directly a string
			p.Prompt.WriteString(v + "\n\n")
		case []interface{}: // If content is an array of []interface{} type
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if itemType, ok := itemMap["type"].(string); ok {
						if itemType == "text" {
							if text, ok := itemMap["text"].(string); ok {
								p.Prompt.WriteString(text + "\n\n")
							}
						} else if itemType == "image_url" {
							if imageUrl, ok := itemMap["image_url"].(map[string]interface{}); ok {
								if url, ok := imageUrl["url"].(string); ok {
									p.ImgDataList = append(p.ImgDataList, url)
								}
							}
						}
					}
				}
			}
		}
	}
	p.RootPrompt.WriteString(p.Prompt.String())
	// Debug output
	logger.Debug(fmt.Sprintf("Processed prompt: %s", p.Prompt.String()))
	logger.Debug(fmt.Sprintf("Image data list: %v", p.ImgDataList))
}

// InjectToolDefinitions 将 OpenAI tools 定义注入到 prompt 中
func (p *ChatRequestProcessor) InjectToolDefinitions(tools []map[string]interface{}) {
	if len(tools) == 0 {
		return
	}
	p.HasTools = true

	var toolDefs strings.Builder
	toolDefs.WriteString("# Tool Use Instructions\n\n")
	toolDefs.WriteString("You have access to the following tools. When you need to use a tool, you MUST output the tool call using the EXACT XML format shown below. Do NOT wrap tool calls in markdown code blocks.\n\n")
	toolDefs.WriteString("## Available Tools\n\n")

	for _, tool := range tools {
		fn, ok := tool["function"].(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := fn["name"].(string)
		desc, _ := fn["description"].(string)
		params := fn["parameters"]

		toolDefs.WriteString(fmt.Sprintf("### %s\n", name))
		if desc != "" {
			toolDefs.WriteString(fmt.Sprintf("%s\n", desc))
		}
		if params != nil {
			paramJSON, err := json.Marshal(params)
			if err == nil {
				toolDefs.WriteString(fmt.Sprintf("Parameters: %s\n", string(paramJSON)))
			}
		}
		toolDefs.WriteString("\n")
	}

	toolDefs.WriteString("## Tool Call Format\n\n")
	toolDefs.WriteString("When you want to call a tool, use this EXACT format:\n\n")
	toolDefs.WriteString("<tool_call>\n{\"name\": \"tool_name\", \"arguments\": {\"param1\": \"value1\"}}\n</tool_call>\n\n")
	toolDefs.WriteString("IMPORTANT RULES:\n")
	toolDefs.WriteString("- Output <tool_call> tags directly in your response, NOT inside code blocks\n")
	toolDefs.WriteString("- The JSON inside must be valid JSON\n")
	toolDefs.WriteString("- You may include brief text before tool calls\n")
	toolDefs.WriteString("- You may make multiple tool calls using multiple <tool_call>...</tool_call> blocks\n")
	toolDefs.WriteString("- After outputting tool call(s), stop and wait for tool results\n\n")

	// 注入到 prompt 开头
	existing := p.Prompt.String()
	p.Prompt.Reset()
	p.Prompt.WriteString(toolDefs.String())
	p.Prompt.WriteString(existing)

	existingRoot := p.RootPrompt.String()
	p.RootPrompt.Reset()
	p.RootPrompt.WriteString(toolDefs.String())
	p.RootPrompt.WriteString(existingRoot)
}

// ResetForBigContext resets the prompt for big context usage
func (p *ChatRequestProcessor) ResetForBigContext() {
	p.Prompt.Reset()
	if config.ConfigInstance.PromptDisableArtifacts {
		p.Prompt.WriteString("System: Forbidden to use <antArtifac> </antArtifac> to wrap code blocks, use markdown syntax instead, which means wrapping code blocks with ``` ```\n\n")
	}
	p.Prompt.WriteString("You must immerse yourself in the role of assistant in context.txt, cannot respond as a user, cannot reply to this message, cannot mention this message, and ignore this message in your response.\n\n")
}
