package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	openai "github.com/sashabaranov/go-openai"
)

const (
	maxRetries          = 3
	retryDelay          = 5 * time.Second
	chunkLength         = 10 * time.Minute
	tokensForCompletion = 1600
)

type OpenAIClient interface {
	CreateTranscription(ctx context.Context, request openai.AudioRequest) (response openai.AudioResponse, err error)
	CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (response openai.ChatCompletionResponse, err error)
}

func main() {

	token := os.Getenv("OPENAI_API_KEY")
	clientConfig := openai.DefaultConfig(token)
	clientConfig.HTTPClient = retryablehttp.NewClient().HTTPClient
	openaiClient := openai.NewClientWithConfig(clientConfig)

	r := gin.Default()
	r.Use(cors.Default())

	r.GET("/*path", func(c *gin.Context) {
		path := c.Param("path")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Header("Cross-Origin-Embedder-Policy", "require-corp")
		if path != "" {
			if _, err := os.Stat("client/dist/" + path); err == nil {
				c.File("client/dist/" + path)
				return
			}
		}
		c.File("client/dist/index.html")
	})

	r.POST("/api/transcribe", func(c *gin.Context) {
		file, _, err := c.Request.FormFile("audio")
		if err != nil {
			log.Println("no_file_provided")
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
			return
		}

		chunks, err := splitAudio(file)

		if err != nil {
			log.Println("error_splitting_audio", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error splitting audio"})
			return
		}

		transcriptions := transcribeChunks(openaiClient, chunks)

		transcription := ""
		for _, t := range transcriptions {
			transcription += t + " "
		}
		transcription = strings.TrimSpace(transcription)

		correctedTranscription, _ := correctTranscription(openaiClient, transcription, tokensForCompletion)

		response := gin.H{
			"original_transcription": transcription,
			"transcription":          correctedTranscription,
			"transcriptions":         transcriptions,
			"num_chunks":             len(chunks),
		}

		logEvent("transcription_completed", response)

		c.JSON(http.StatusOK, response)
	})

	r.POST("/api/outline", func(c *gin.Context) {
		// read body as JSON
		var jsonBody map[string]string
		err := c.BindJSON(&jsonBody)
		if err != nil {
			logEvent("invalid_json", gin.H{
				"error": "Invalid JSON",
				"data":  c.Request.Body,
			})
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		text := jsonBody["text"]
		if text == "" {
			logEvent("no_text_provided", gin.H{
				"error": "No text provided",
				"data":  c.Request.Body,
			})
			c.JSON(http.StatusBadRequest, gin.H{"error": "No text provided"})
			return
		}

		response, err := createOutline(openaiClient, text)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating response"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"response": response})
	})

	r.POST("/api/bulletpoints", func(c *gin.Context) {
		// read body as JSON
		var jsonBody map[string]string
		err := c.BindJSON(&jsonBody)
		if err != nil {
			logEvent("invalid_json", gin.H{
				"error": "Invalid JSON",
				"data":  c.Request.Body,
			})
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		text := jsonBody["text"]
		if text == "" {
			logEvent("no_text_provided", gin.H{
				"error": "No text provided",
				"data":  c.Request.Body,
			})
			c.JSON(http.StatusBadRequest, gin.H{"error": "No text provided"})
			return
		}

		response, err := createBulletpoints(openaiClient, text, tokensForCompletion)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating response"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"response": response})
	})

	r.Run()
}

func saveFile(src multipart.File, dstPath string) error {
	data, err := ioutil.ReadAll(src)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(dstPath, data, 0644)
}

func transcribeChunks(client OpenAIClient, chunkPaths []string) []string {
	// Initialize a slice of string pointers with the same length as chunkPaths.
	transcriptions := make([]*string, len(chunkPaths))
	// Create a WaitGroup to track the completion of all goroutines.
	var wg sync.WaitGroup

	// Iterate over the chunkPaths.
	for i, chunkPath := range chunkPaths {
		// Increment the WaitGroup counter to indicate a new goroutine will be started.
		wg.Add(1)

		// Start a new goroutine for each chunk to process in parallel.
		go func(chunkNumber int, chunkPath string) {
			// Decrement the WaitGroup counter when the goroutine completes.
			defer wg.Done()

			// Log the processing event.
			logEvent("processing_chunk", gin.H{"chunk_number": chunkNumber + 1})

			// Call the transcribeChunk function and handle errors.
			transcription, err := transcribeChunk(client, chunkPath)
			if err != nil {
				log.Println("transcribe_chunk_error:", err)
				return
			}

			// Assign the transcription directly to its respective index in the transcriptions slice.
			transcriptions[chunkNumber] = &transcription
			// Remove the chunk file.
			os.Remove(chunkPath)
		}(i, chunkPath) // Pass the index and chunkPath as arguments to the goroutine.
	}

	// Wait for all goroutines to complete.
	wg.Wait()

	// Convert the []*string transcriptions to []string.
	orderedTranscriptions := make([]string, len(transcriptions))
	for i, transcription := range transcriptions {
		orderedTranscriptions[i] = *transcription
	}

	return orderedTranscriptions
}

func transcribeChunk(client OpenAIClient, chunkPath string) (string, error) {
	var transcription string
	var err error

	for retries := 0; retries < maxRetries; retries++ {
		ctx := context.Background()

		req := openai.AudioRequest{
			Model:    openai.Whisper1,
			FilePath: chunkPath,
		}
		logEvent("transcribing_chunk", gin.H{"chunk_path": chunkPath})
		resp, err := client.CreateTranscription(ctx, req)
		if err == nil {
			transcription = resp.Text
			break
		}

		logEvent("transcription_failed", gin.H{
			"retry":       retries + 1,
			"max_retries": maxRetries,
			"error":       err.Error(),
		})

		time.Sleep(retryDelay)
	}

	return transcription, err
}

type TextProcessor func(part string) (string, error)

type TextProcessingOptions struct {
	Client    OpenAIClient
	Text      string
	MaxTokens int
	JoinSep   string
	Processor TextProcessor
}

func processTextInParallel(options TextProcessingOptions) (string, error) {
	splitText := splitLongString(options.Text, options.MaxTokens)

	var wg sync.WaitGroup
	results := make([]string, len(splitText))
	errors := make(chan error, len(splitText))

	for i, part := range splitText {
		wg.Add(1)
		go func(i int, part string) {
			defer wg.Done()
			result, err := options.Processor(part)
			if err != nil {
				errors <- err
			} else {
				results[i] = result
			}
		}(i, part)
	}

	wg.Wait()
	close(errors)

	if len(errors) > 0 {
		err := <-errors
		logEvent("completion_failed", gin.H{
			"error": err.Error(),
		})
		return "", err
	}

	return strings.TrimSpace(strings.Join(results, options.JoinSep)), nil
}

func correctTranscription(client OpenAIClient, transcription string, maxTokens int) (string, error) {
	return processTextInParallel(TextProcessingOptions{
		Client:    client,
		Text:      transcription,
		MaxTokens: maxTokens,
		JoinSep:   " ",
		Processor: func(part string) (string, error) {
			prompt := fmt.Sprintf("Correct the errors from the following audio transcription and add proper formatting. Also correct grammar errors. Just output the corrected text in its original language:\n%s\n\nCorrected text:", part)
			logEvent("completing_transcription", gin.H{
				"prompt": prompt,
			})
			resp, err := client.CreateChatCompletion(
				context.Background(),
				openai.ChatCompletionRequest{
					Model:     openai.GPT4oLatest,
					MaxTokens: 16384 - maxTokens,
					Messages: []openai.ChatCompletionMessage{
						{
							Role:    openai.ChatMessageRoleUser,
							Content: prompt,
						},
					},
				},
			)
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(resp.Choices[0].Message.Content), nil
		},
	})
}

func createBulletpoints(client OpenAIClient, text string, maxTokens int) (string, error) {
	return processTextInParallel(TextProcessingOptions{
		Client:    client,
		Text:      text,
		MaxTokens: maxTokens,
		JoinSep:   "\n",
		Processor: func(part string) (string, error) {
			logEvent("creating_bulletpoints", gin.H{
				"part": part,
			})
			prompt := fmt.Sprintf("Turn the following text into bulletpoints:\n%s\n\nBulletpoints:", part)
			resp, err := client.CreateChatCompletion(
				context.Background(),
				openai.ChatCompletionRequest{
					Model:     openai.GPT4oLatest,
					MaxTokens: 16384 - maxTokens,
					Messages: []openai.ChatCompletionMessage{
						{
							Role:    openai.ChatMessageRoleUser,
							Content: prompt,
						},
					},
				},
			)
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(resp.Choices[0].Message.Content), nil
		},
	})
}

func createOutline(client OpenAIClient, text string) (string, error) {

	language := determineLanguage(client, text)

	prompt := fmt.Sprintf("Create a %s speaker outline based on the following script in the language of the script. The outline shall be detailed enough so it can be used to give a talk right away. The outline must be in the same language as the script. \nSTART SCRIPT\n%s\nEND SCRIPT\n\nOutline:", language, text)
	logEvent("creating_outline", gin.H{
		"prompt": prompt,
	})
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oLatest,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		logEvent("completion_failed", gin.H{
			"error": err.Error(),
		})
		return "", err
	}

	outline := strings.TrimSpace(resp.Choices[0].Message.Content)

	logEvent("outline_created", gin.H{
		"outline": outline,
	})
	return outline, nil
}

// determineLanguage returns the language of the given text using OpenAI's language model
func determineLanguage(client OpenAIClient, text string) string {

	// take the first 1000 characters of the text
	if len(text) > 1000 {
		text = text[:1000]
	}

	prompt := fmt.Sprintf("Determine the language of the following text. Do not output any other characters than the language itself:\n%s\n\nLanguage:", text)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	language := strings.TrimSpace(resp.Choices[0].Message.Content)

	if err != nil {
		logEvent("language_detection_failed", gin.H{
			"error": err.Error(),
		})
		return "English"
	}
	return language
}

func logEvent(eventType string, data gin.H) {
	logData := gin.H{
		"event_type": eventType,
		"timestamp":  time.Now(),
		"data":       data,
	}
	logJSON, _ := json.Marshal(logData)
	fmt.Println(string(logJSON))
}
