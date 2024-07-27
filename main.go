package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/plugins/googleai"
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
	godotenv.Load()
	title = os.Getenv("TITLE")
	if title == "" {
		title = "unset"
	}

	personality = os.Getenv("PERSONALITY")
	if personality == "" {
		personality = "unset"
	}

	postEndpoint = os.Getenv("POST_ENDPOINT")
	if postEndpoint == "" {
		postEndpoint = "http://192.168.0.100:5050/webhook"
	}
}

func generateJokes() []string {
	ctx := context.Background()

	if err := googleai.Init(ctx, nil); err != nil {
		log.Fatal(err)
	}

	m := googleai.Model("gemini-1.5-flash")
	if m == nil {
		return nil
	}

	requestText := fmt.Sprintf("Your personality is: %s. Provide a list of 20 '|' (pipe) separated jokes tightly in line with the personality. Format: joke1|joke2|joke3|joke4| ...", personality)

	resp, err := m.Generate(ctx,
		ai.NewGenerateRequest(
			&ai.GenerationCommonConfig{Temperature: 1},
			ai.NewUserTextMessage(requestText)),
		nil)
	if err != nil {
		log.Fatal(err)
	}

	text, err := resp.Text()
	if err != nil {
		log.Fatal(err)
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

			resp, err := http.Post(postEndpoint, "application/json", strings.NewReader(fmt.Sprintf(`{"joke": "%s"}`, joke)))
			if err != nil {
				fmt.Println("Failed to post joke:", err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				fmt.Println("Successfully posted joke:", joke)
			} else {
				fmt.Println("Failed to post joke. Status code:", resp.StatusCode)
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
