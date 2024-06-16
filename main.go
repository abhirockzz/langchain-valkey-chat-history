package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	_ "embed"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms/bedrock"
	"github.com/tmc/langchaingo/memory"
	"github.com/tmc/langchaingo/outputparser"
	"github.com/tmc/langchaingo/prompts"
)

var chain chains.LLMChain
var promptsTemplate prompts.PromptTemplate
var llm *bedrock.LLM
var client *redis.Client

const template = "{{.chat_history}}\n{{.human_input}}"
const maxTokensForClaudeV3Sonnet = 4096

func init() {

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	brc := bedrockruntime.NewFromConfig(cfg)

	llm, err = bedrock.New(bedrock.WithClient(brc), bedrock.WithModel(bedrock.ModelAnthropicClaudeV3Sonnet))
	//llm.CallbacksHandler = callbacks.LogHandler{}

	if err != nil {
		log.Fatal(err)
	}

	client = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	err = client.Ping(context.Background()).Err()
	if err != nil {
		log.Fatal(err)
	}

	promptsTemplate = prompts.NewPromptTemplate(
		template,
		[]string{"chat_history", "human_input"},
	)

	sessionID := uuid.NewString()

	fmt.Println("\nchat session ID:", sessionID)

	chatMemory := memory.NewConversationBuffer(
		memory.WithMemoryKey("chat_history"),
		memory.WithChatHistory(&ValkeyMessageHistory{
			sessionID:  sessionID,
			sessionTTL: 300,
			client:     client,
		}),
	)

	chain = chains.LLMChain{
		Prompt:       promptsTemplate,
		LLM:          llm,
		Memory:       chatMemory,
		OutputParser: outputparser.NewSimple(),
		OutputKey:    "text",
	}

}

func main() {

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\n[You]: ")
		message, _ := reader.ReadString('\n')
		message = strings.TrimSpace(message)

		fmt.Print("[AI]: ")

		_, err := chains.Call(context.Background(), chain, map[string]any{"human_input": message}, chains.WithMaxTokens(maxTokensForClaudeV3Sonnet), chains.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {

			fmt.Print(string(chunk))
			return nil
		}))

		if err != nil {
			log.Fatal(err)
		}
	}
}
