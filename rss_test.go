package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestFilterNewJobs(t *testing.T) {
	now := time.Now()
	// threshold as one hour ago.
	threshold := now.Add(-1 * time.Hour)

	// job posted 2 hours ago.
	oldJob := &gofeed.Item{
		Title: "Old Job",
		Link:  "/old",
		// Simulated published timestamp.
		Published: "Old timestamp",
	}
	oldJobTime := now.Add(-2 * time.Hour)
	oldJob.PublishedParsed = &oldJobTime

	// job posted 30 minutes ago.
	newJob := &gofeed.Item{
		Title:     "New Job",
		Link:      "/new",
		Published: "New timestamp",
	}
	newJobTime := now.Add(-30 * time.Minute)
	newJob.PublishedParsed = &newJobTime

	items := []*gofeed.Item{oldJob, newJob}
	filtered := filterNewJobs(items, threshold)

	if len(filtered) != 1 {
		t.Errorf("Expected 1 new job, got %d", len(filtered))
	}
	if filtered[0].Title != "New Job" {
		t.Errorf("Expected job title 'New Job', got '%s'", filtered[0].Title)
	}

	fmt.Println("New Job Detected:")
	fmt.Println("🔹", filtered[0].Title)
	fmt.Println("📎", filtered[0].Link)
	fmt.Println("🕒", filtered[0].Published)
}
