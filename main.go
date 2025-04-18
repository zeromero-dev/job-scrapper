package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/segmentio/kafka-go"
)

var (
	kafkaWriter *kafka.Writer
	feeds       = []string{
		"https://jobs.dou.ua/vacancies/feeds/?category=Golang&exp=0-1",
		"https://jobs.dou.ua/vacancies/feeds/?exp=1-3&category=Golang",
	}
)

func initKafka() {
	kafkaWriter = kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "job_notifications",
		Balancer: &kafka.LeastBytes{},
	})
}

func closeKafka() {
	if kafkaWriter != nil {
		kafkaWriter.Close()
	}
}

func filterNewJobs(items []*gofeed.Item, threshold time.Time) []*gofeed.Item {
	var newJobs []*gofeed.Item
	for _, item := range items {
		if item.PublishedParsed != nil && item.PublishedParsed.After(threshold) {
			newJobs = append(newJobs, item)
		}
	}
	return newJobs
}

func fetchFeed(url string, ch chan<- []*gofeed.Item, wg *sync.WaitGroup) {
	defer wg.Done()

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Failed to fetch %s: %v", url, err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read body for %s: %v", url, err)
		return
	}

	fp := gofeed.NewParser()
	feed, err := fp.ParseString(string(body))
	if err != nil {
		log.Printf("Failed to parse feed from %s: %v", url, err)
		return
	}

	if len(feed.Items) > 0 {
		ch <- feed.Items
	}
}

func collectFeeds() []*gofeed.Item {
	var wg sync.WaitGroup
	itemChan := make(chan []*gofeed.Item, len(feeds))

	for _, url := range feeds {
		wg.Add(1)
		go fetchFeed(url, itemChan, &wg)
	}

	wg.Wait()
	close(itemChan)

	var allItems []*gofeed.Item
	for items := range itemChan {
		allItems = append(allItems, items...)
	}
	return allItems
}

func formatJobs(items []*gofeed.Item) string {
	var lines []string
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("🔹 %s\n📎 %s\n🕒 %s", item.Title, item.Link, item.Published))
	}
	return strings.Join(lines, "\n\n")
}

func handleNewJobs(w http.ResponseWriter, r *http.Request) {
	allItems := collectFeeds()
	newJobs := filterNewJobs(allItems, time.Now().Add(-1*time.Hour))

	if len(newJobs) == 0 {
		http.Error(w, "No new vacancies found.", http.StatusNotFound)
		return
	}

	msg := formatJobs(newJobs)

	// Send to Kafka
	err := kafkaWriter.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte("job_notification"),
			Value: []byte(msg),
		},
	)
	if err != nil {
		log.Printf("Kafka error: %v", err)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(msg))
}

func handleAllJobs(w http.ResponseWriter, r *http.Request) {
	allItems := collectFeeds()
	if len(allItems) == 0 {
		http.Error(w, "No vacancies found.", http.StatusNotFound)
		return
	}

	msg := formatJobs(allItems)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(msg))
}

func main() {
	initKafka()
	defer closeKafka()

	http.HandleFunc("/new", handleNewJobs)
	http.HandleFunc("/all", handleAllJobs)

	log.Println("Server started at :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
