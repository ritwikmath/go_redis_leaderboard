package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/redis/go-redis/v9"
)

var (
	redis_client *redis.Client

	ctx = context.Background()

	ch = make(chan bool)
)

func initializeRedisConnection() {
	redis_client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // No password set
		DB:       0,  // Use default DB
		Protocol: 2,  // Connection protocol
	})
}

func updateScore(w http.ResponseWriter, r *http.Request) {

	var req struct {
		Name  string  `json:"name"`
		Score float64 `json:"score"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	redis_client.ZAdd(ctx, "leaderboard", redis.Z{Score: req.Score, Member: req.Name})
	ch <- true
	fmt.Fprint(w, "Done")
}

func leaderboardSSEHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	for {
		loaded := <-ch
		if loaded {
			leaderboard, err := redis_client.ZRevRangeByScoreWithScores(ctx, "leaderboard", &redis.ZRangeBy{
				Min: "-inf",
				Max: "+inf",
			}).Result()
			if err != nil {
				fmt.Fprint(w, "Failed to get score")
				w.(http.Flusher).Flush()
			}

			var frontendData string = "data:["
			for _, result := range leaderboard {
				output := fmt.Sprintf(`{"name":"%v","score":%v},`, result.Member, result.Score)
				frontendData += output
			}
			frontendData = strings.TrimSuffix(frontendData, ",")
			frontendData += "]\n\n"

			fmt.Fprint(w, frontendData)
			w.(http.Flusher).Flush()
		}
	}
}

func main() {
	initializeRedisConnection()

	fs := http.FileServer(http.Dir("./static"))

	http.Handle("/", fs)
	http.HandleFunc("/update-score", updateScore)
	http.HandleFunc("/leaderboard/sse", leaderboardSSEHandler)

	http.ListenAndServe("127.0.0.1:8088", nil)
}
