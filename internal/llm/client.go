package llm

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type StreamChunk struct {
	Content string
	Done    bool
	Err     error
}

type Client struct {
	chatModel model.ChatModel
}

type ClientConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

func NewClient(cfg *ClientConfig) (*Client, error) {
	chatModel, err := openai.NewChatModel(context.Background(), &openai.ChatModelConfig{
		APIKey: cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 客户端失败: %w", err)
	}

	return &Client{
		chatModel: chatModel,
	}, nil
}

func (c *Client) Chat(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	resp, err := c.chatModel.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("生成回复失败: %w", err)
	}
	return resp, nil
}

func (c *Client) StreamChat(ctx context.Context, messages []*schema.Message) <-chan StreamChunk {
	chunkCh := make(chan StreamChunk)

	go func() {
		defer close(chunkCh)

		stream, err := c.chatModel.Stream(ctx, messages)
		if err != nil {
			chunkCh <- StreamChunk{Err: fmt.Errorf("创建流式对话失败: %w", err)}
			return
		}
		defer stream.Close()

		for {
			msg, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					chunkCh <- StreamChunk{Done: true}
					return
				}
				chunkCh <- StreamChunk{Err: fmt.Errorf("接收流式数据失败: %w", err)}
				return
			}
			if msg != nil && msg.Content != "" {
				chunkCh <- StreamChunk{Content: msg.Content}
			}
		}
	}()

	return chunkCh
}


