package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/mmcdole/gofeed"
)

func fetchFeed(url string, ch chan<- []*gofeed.Item, wg *sync.WaitGroup) {
	defer wg.Done()

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Failed to fetch %s: %v\n", url, err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read body for %s: %v\n", url, err)
		return
	}

	fp := gofeed.NewParser()
	feed, err := fp.ParseString(string(body))
	if err != nil {
		log.Printf("Failed to parse feed from %s: %v\n", url, err)
		return
	}

	if len(feed.Items) == 0 {
		fmt.Printf("❌ No vacancies in %s\n", url)
		return
	}

	ch <- feed.Items
}

func main() {
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

	for _, item := range allItems {
		fmt.Println()
		fmt.Println("🔹", item.Title)
		fmt.Println("📎", item.Link)
		fmt.Println("🕒", item.Published)
		fmt.Println()
	}
}
