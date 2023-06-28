package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/spf13/viper"
	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/token"
	"github.com/tencent-connect/botgo/websocket"
)

func main() {
	v, err := InitConfig()
	if err != nil {
		log.Fatal(err)
	}
	appId := v.GetUint64("qqbot.app_id")
	accessToken := v.GetString("qqbot.access_token")
	token1 := token.BotToken(appId, accessToken)
	api := botgo.NewSandboxOpenAPI(token1).WithTimeout(3 * time.Second)
	ctx := context.Background()
	ws, err := api.WS(ctx, nil, "")
	if err != nil {
		log.Printf("%+v, err:%v", ws, err)
	}

	// 监听哪类事件就需要实现哪类的 handler，定义：websocket/event_handler.go
	var atMessage event.ATMessageEventHandler = func(event *dto.WSPayload, data *dto.WSATMessageData) error {
		fmt.Println("EVENT: ", event, data)
		response := NewOpenAIResponse(v, data.Content)
		if response != "" {
			message, err := api.PostMessage(ctx, data.ChannelID, &dto.MessageToCreate{
				MsgID:   "", //如果未空则表示主动消息
				Content: response,
			})
			if err != nil {
				log.Printf("PostMessage, err:%v", err)
			}
			fmt.Println(message)
		}

		return nil
	}
	intent := websocket.RegisterHandlers(atMessage)
	// 启动 session manager 进行 ws 连接的管理，如果接口返回需要启动多个 shard 的连接，这里也会自动启动多个
	botgo.NewSessionManager().Start(ws, token1, &intent)
}

func NewOpenAIResponse(viper *viper.Viper, input string) (output string) {
	key := viper.GetString("openai.key")
	proxy := viper.GetString("openai.proxy")
	config := openai.DefaultConfig(key)

	if proxy != "" {
		proxyURL, err := url.Parse("http://127.0.0.1:7890")
		if err != nil {
			fmt.Printf("proxyURL error: %v\n", err)
			return
		}
		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
		config.HTTPClient = &http.Client{
			Transport: transport,
		}
	}

	client := openai.NewClientWithConfig(config)
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: input,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return
	}
	fmt.Println(resp.Choices[0].Message.Content)
	output = resp.Choices[0].Message.Content
	return
}

func InitConfig() (*viper.Viper, error) {
	v := viper.New()
	v.AddConfigPath("./")
	v.SetConfigType("toml")
	v.SetConfigName("config.toml")

	if err := v.ReadInConfig(); err == nil {
		log.Printf("use config file -> %s\n", v.ConfigFileUsed())
	} else {
		return nil, err
	}
	return v, nil
}
