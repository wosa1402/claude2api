package model

import (
	"claude2api/logger"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ChatCompletionRequest struct {
	Model    string                   `json:"model"`
	Messages []map[string]interface{} `json:"messages"`
	Stream   bool                     `json:"stream"`
	Tools    []map[string]interface{} `json:"tools,omitempty"`
}

// OpenAISrteamResponse 定义 OpenAI 的流式响应结构
type OpenAISrteamResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []StreamChoice `json:"choices"`
}

// Choice 结构表示 OpenAI 返回的单个选项
type StreamChoice struct {
	Index        int         `json:"index"`
	Delta        Delta       `json:"delta"`
	Logprobs     interface{} `json:"logprobs,omitempty"`
	FinishReason interface{} `json:"finish_reason"`
}

type NoStreamChoice struct {
	Index        int         `json:"index"`
	Message      Message     `json:"message"`
	Logprobs     interface{} `json:"logprobs"`
	FinishReason string      `json:"finish_reason"`
}

// Delta 结构用于存储返回的文本内容
type Delta struct {
	Role      string          `json:"role,omitempty"`
	Content   interface{}     `json:"content,omitempty"`
	ToolCalls []DeltaToolCall `json:"tool_calls,omitempty"`
}

type Message struct {
	Role       string        `json:"role"`
	Content    interface{}   `json:"content"`
	Refusal    interface{}   `json:"refusal"`
	Annotation []interface{} `json:"annotation"`
	ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
}

// ToolCall 用于非流式响应中的工具调用
type ToolCall struct {
	Index    int              `json:"index"`
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// DeltaToolCall 用于流式响应中的工具调用
type DeltaToolCall struct {
	Index    int                   `json:"index"`
	ID       string                `json:"id,omitempty"`
	Type     string                `json:"type,omitempty"`
	Function DeltaToolCallFunction `json:"function"`
}

type DeltaToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type OpenAIResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []NoStreamChoice `json:"choices"`
	Usage   Usage            `json:"usage"`
}
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func NewCompletionID() string {
	return "chatcmpl-" + uuid.New().String()
}

func NewToolCallID() string {
	return "call_" + uuid.New().String()
}

func ReturnOpenAIResponse(text string, stream bool, gc *gin.Context, completionID string) error {
	if stream {
		return streamRespose(text, gc, completionID)
	} else {
		return noStreamResponse(text, gc, completionID)
	}
}

func streamRespose(text string, gc *gin.Context, completionID string) error {
	openAIResp := &OpenAISrteamResponse{
		ID:      completionID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   "claude-3-7-sonnet-20250219",
		Choices: []StreamChoice{
			{
				Index: 0,
				Delta: Delta{
					Content: text,
				},
				FinishReason: nil,
			},
		},
	}

	jsonBytes, err := json.Marshal(openAIResp)
	if err != nil {
		logger.Error(fmt.Sprintf("Error marshalling JSON: %v", err))
		return err
	}
	jsonBytes = append([]byte("data: "), jsonBytes...)
	jsonBytes = append(jsonBytes, []byte("\n\n")...)

	gc.Writer.Write(jsonBytes)
	gc.Writer.Flush()
	return nil
}

func noStreamResponse(text string, gc *gin.Context, completionID string) error {
	openAIResp := &OpenAIResponse{
		ID:      completionID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "claude-3-7-sonnet-20250219",
		Choices: []NoStreamChoice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: text,
				},
				Logprobs:     nil,
				FinishReason: "stop",
			},
		},
	}

	gc.JSON(200, openAIResp)
	return nil
}

func StreamFinishResponse(gc *gin.Context, completionID string, finishReason string) error {
	openAIResp := &OpenAISrteamResponse{
		ID:      completionID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   "claude-3-7-sonnet-20250219",
		Choices: []StreamChoice{
			{
				Index:        0,
				Delta:        Delta{},
				FinishReason: finishReason,
			},
		},
	}

	jsonBytes, err := json.Marshal(openAIResp)
	if err != nil {
		logger.Error(fmt.Sprintf("Error marshalling JSON: %v", err))
		return err
	}
	jsonBytes = append([]byte("data: "), jsonBytes...)
	jsonBytes = append(jsonBytes, []byte("\n\n")...)

	gc.Writer.Write(jsonBytes)
	gc.Writer.Flush()
	return nil
}

// StreamToolCallStartResponse 发送工具调用的第一个 chunk（包含 id、type、name）
func StreamToolCallStartResponse(toolCallIndex int, id string, name string, arguments string, gc *gin.Context, completionID string) error {
	openAIResp := &OpenAISrteamResponse{
		ID:      completionID,
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   "claude-3-7-sonnet-20250219",
		Choices: []StreamChoice{
			{
				Index: 0,
				Delta: Delta{
					Role: "assistant",
					ToolCalls: []DeltaToolCall{
						{
							Index: toolCallIndex,
							ID:    id,
							Type:  "function",
							Function: DeltaToolCallFunction{
								Name:      name,
								Arguments: arguments,
							},
						},
					},
				},
				FinishReason: nil,
			},
		},
	}

	jsonBytes, err := json.Marshal(openAIResp)
	if err != nil {
		logger.Error(fmt.Sprintf("Error marshalling JSON: %v", err))
		return err
	}
	jsonBytes = append([]byte("data: "), jsonBytes...)
	jsonBytes = append(jsonBytes, []byte("\n\n")...)

	gc.Writer.Write(jsonBytes)
	gc.Writer.Flush()
	return nil
}

// NoStreamToolCallResponse 发送包含 tool_calls 的非流式响应
func NoStreamToolCallResponse(toolCalls []ToolCall, textContent string, gc *gin.Context, completionID string) error {
	var content interface{}
	if textContent != "" {
		content = textContent
	}
	openAIResp := &OpenAIResponse{
		ID:      completionID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "claude-3-7-sonnet-20250219",
		Choices: []NoStreamChoice{
			{
				Index: 0,
				Message: Message{
					Role:      "assistant",
					Content:   content,
					ToolCalls: toolCalls,
				},
				Logprobs:     nil,
				FinishReason: "tool_calls",
			},
		},
	}

	gc.JSON(200, openAIResp)
	return nil
}

func sendSSEChunk(data []byte, gc *gin.Context) {
	chunk := append([]byte("data: "), data...)
	chunk = append(chunk, []byte("\n\n")...)
	gc.Writer.Write(chunk)
	gc.Writer.Flush()
}
