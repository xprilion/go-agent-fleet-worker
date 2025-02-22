package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

var (
	jokes        []string
	title        string
	personality  string
	postEndpoint string
	jokesMutex   sync.Mutex
)

func init() {
	godotenv.Load() // Load environment variables from .env file
	title = os.Getenv("TITLE")
	if title == "" {
		title = "unset"
	}

	personality = os.Getenv("PERSONALITY")
	if personality == "" {
		personality = "You are a funny and engaging comedian. You are also a bit of a nerd and like to talk about technology and science."
	}

	postEndpoint = os.Getenv("POST_ENDPOINT")
	if postEndpoint == "" {
		postEndpoint = "https://agent-fleet-ui.web.app/api/webhook"
	}
}

func generateJokes() []string {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable not set")
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=%s", apiKey)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{
						"text": fmt.Sprintf("Your personality is: %s. Provide a list of 20 '|' (pipe) separated jokes tightly in line with the personality, only safe for work jokes. Format: joke1|joke2|joke3|joke4| ...", personality),
					},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		log.Fatal("Failed to marshal request body:", err)
		return nil
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Fatal("Failed to create request:", err)
		return nil
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Failed to make request:", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read response body:", err)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Request failed with status code %d. Response: %s", resp.StatusCode, string(body))
		return nil
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Fatal("Failed to unmarshal response:", err)
		return nil
	}

	candidates, ok := response["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		log.Fatal("No candidates in response")
		return nil
	}

	content, ok := candidates[0].(map[string]interface{})["content"].(map[string]interface{})
	if !ok {
		log.Fatal("Invalid content structure")
		return nil
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		log.Fatal("Invalid parts structure")
		return nil
	}

	text, ok := parts[0].(map[string]interface{})["text"].(string)
	if !ok {
		log.Fatal("Invalid text structure")
		return nil
	}

	return splitJokes(text)
}

func splitJokes(jokes string) []string {
	return strings.Split(jokes, "|")
}

func postJokePeriodically() {
	for {
		jokesMutex.Lock()
		if len(jokes) > 0 {
			joke := jokes[rand.Intn(len(jokes))]
			jokesMutex.Unlock()

			// Replace newline characters with their escaped version
			escapedJoke := strings.ReplaceAll(joke, "\n", "\\n")

			currentTime := time.Now().UnixMilli()
			payload := map[string]interface{}{
				"collectionName": "pings-aicd-delhi",
				"data": map[string]interface{}{
					"name":      title,
					"message":   escapedJoke,
					"timestamp": currentTime,
				},
			}

			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				fmt.Println("Failed to marshal JSON payload:", err)
				continue
			}

			req, err := http.NewRequest("POST", postEndpoint, bytes.NewBuffer(payloadBytes))
			if err != nil {
				fmt.Println("Failed to create new POST request:", err)
				continue
			}

			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("Failed to post joke:", err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				fmt.Println("Successfully posted joke:", joke)
			} else {
				fmt.Printf("Failed to post joke. Status code: %d. Response content: %s\n", resp.StatusCode, resp.Status)
			}
		} else {
			jokesMutex.Unlock()
			fmt.Println("No joke found.")
		}
		time.Sleep(5 * time.Second)
	}
}

func getJoke(w http.ResponseWriter, r *http.Request) {
	jokesMutex.Lock()
	defer jokesMutex.Unlock()

	if len(jokes) > 0 {
		joke := jokes[rand.Intn(len(jokes))]
		fmt.Fprintf(w, joke)
	} else {
		fmt.Fprintf(w, "No joke found.")
	}
}

func main() {
	jokes = generateJokes()
	fmt.Println("Jokes generated:", jokes)

	go postJokePeriodically()

	http.HandleFunc("/", getJoke)

	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}
	fmt.Printf("Server started at :%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Println("Failed to start server:", err)
	}
}
