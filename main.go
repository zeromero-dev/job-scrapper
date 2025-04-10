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

var kafkaWriter *kafka.Writer

// initKafka creates a Kafka writer to send messages to our topic.
func initKafka() {
	kafkaWriter = kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{"localhost:9092"}, // adjust if needed
		Topic:    "job_notifications",
		Balancer: &kafka.LeastBytes{},
	})
}

// closeKafka closes the Kafka writer connection.
func closeKafka() {
	if kafkaWriter != nil {
		kafkaWriter.Close()
	}
}

// filterNewJobs returns only those items published after the given threshold.
func filterNewJobs(items []*gofeed.Item, threshold time.Time) []*gofeed.Item {
	var newJobs []*gofeed.Item
	for _, item := range items {
		// Debug info
		fmt.Println("DEBUG:", item.Title, item.Link)
		// Use PublishedParsed to check timestamp.
		if item.PublishedParsed != nil && item.PublishedParsed.After(threshold) {
			newJobs = append(newJobs, item)
		}
	}
	return newJobs
}

// fetchFeed retrieves and parses the feed from the given URL.
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

	if len(feed.Items) == 0 {
		fmt.Printf("❌ No vacancies in %s\n", url)
		return
	}

	ch <- feed.Items
}

// checkFeeds fetches the feeds, filters new vacancies, and sends the formatted message to Kafka.
func checkFeeds() {
	feeds := []string{
		"https://jobs.dou.ua/vacancies/feeds/?category=Golang&exp=0-1",
		"https://jobs.dou.ua/vacancies/feeds/?exp=1-3&category=Golang",
	}

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

	// Define new vacancy threshold: items published within the last hour.
	newThreshold := time.Now().Add(-1 * time.Hour)
	newJobs := filterNewJobs(allItems, newThreshold)
	if len(newJobs) == 0 {
		log.Println("No new vacancies found.")
		return
	}

	// Build a message for the new vacancy(ies).
	var newJobsDetails []string
	for _, item := range newJobs {
		newJobsDetails = append(newJobsDetails, fmt.Sprintf("🔹 %s\n📎 %s\n🕒 %s", item.Title, item.Link, item.Published))
	}
	newJobsMsg := strings.Join(newJobsDetails, "\n\n")

	// Build a message for all available vacancies.
	var allJobsDetails []string
	for _, item := range allItems {
		allJobsDetails = append(allJobsDetails, fmt.Sprintf("🔹 %s\n📎 %s\n🕒 %s", item.Title, item.Link, item.Published))
	}
	allJobsMsg := strings.Join(allJobsDetails, "\n\n")

	// Construct the final Kafka message.
	finalMsg := fmt.Sprintf("New vacancy just dropped!\n\n%s\n\nAlso if you missed previous:\n\n%s", newJobsMsg, allJobsMsg)

	// Send the message to Kafka.
	log.Println("Sending message to Kafka...")
	err := kafkaWriter.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte("job_notification"),
			Value: []byte(finalMsg),
		},
	)
	if err != nil {
		log.Printf("Failed to send Kafka message: %v", err)
	} else {
		log.Println("Kafka message sent successfully.")
	}
}

func main() {
	initKafka()
	defer closeKafka()

	// Immediately check feeds once.
	checkFeeds()

	// Set up a ticker to check feeds every hour.
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		checkFeeds()
	}
}
