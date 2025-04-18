// Save this as main_test.go in the same directory as your main.go file

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

const mockFeed = `
<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Job Feed</title>
    <link>https://example.com/jobs</link>
    <description>Test job listings</description>
    <item>
      <title>Senior Golang Developer</title>
      <link>https://example.com/job/1</link>
      <description>Exciting opportunity for a Golang developer</description>
      <pubDate>%s</pubDate>
    </item>
    <item>
      <title>Golang Engineer</title>
      <link>https://example.com/job/2</link>
      <description>Join our growing team</description>
      <pubDate>%s</pubDate>
    </item>
  </channel>
</rss>
`

func TestEmailNotification(t *testing.T) {
	// Create future timestamps for the job listings
	futureTime1 := time.Now().Add(1 * time.Hour).Format(time.RFC1123Z)
	futureTime2 := time.Now().Add(2 * time.Hour).Format(time.RFC1123Z)

	// Format the mock feed with future timestamps
	feedContent := fmt.Sprintf(mockFeed, futureTime1, futureTime2)

	// Create a mock server that returns our feed
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprintln(w, feedContent)
	}))
	defer mockServer.Close()

	// Temporarily replace the feeds with our mock server
	originalFeeds := feeds
	defer func() { feeds = originalFeeds }()
	feeds = []string{mockServer.URL}

	// Reset lastCheckedTime to ensure our jobs are "new"
	mu.Lock()
	lastCheckedTime = time.Now().Add(-1 * time.Hour)
	mu.Unlock()

	// Create a recorder to capture the response
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/new", nil)

	// Call the handler
	handleNewJobs(recorder, req)

	resp := recorder.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	bodyBytes := make([]byte, recorder.Body.Len())
	recorder.Body.Read(bodyBytes)
	body := string(bodyBytes)

	assert.True(t, strings.Contains(body, "Senior Golang Developer"))
	assert.True(t, strings.Contains(body, "Golang Engineer"))

	// The test above will trigger email sending as a side effect.
	// In a real test environment, you might want to mock the email sending function
	// to verify it was called without actually sending emails.

	fmt.Println("Test completed. Check your email inbox for test notifications.")
}

// TestMockEmailSending mocks the email sending function to verify it works
// without actually sending emails
func TestMockEmailSending(t *testing.T) {
	originalSendEmail := sendEmailNotification

	emailSent := false
	emailSubject := ""
	emailBody := ""

	// Replace with mock function
	sendEmailNotification = func(subject, body string) error {
		emailSent = true
		emailSubject = subject
		emailBody = body
		return nil
	}

	defer func() { sendEmailNotification = originalSendEmail }()

	// Create future timestamps for the job listings
	futureTime1 := time.Now().Add(1 * time.Hour).Format(time.RFC1123Z)
	futureTime2 := time.Now().Add(2 * time.Hour).Format(time.RFC1123Z)

	// Format the mock feed with future timestamps
	feedContent := fmt.Sprintf(mockFeed, futureTime1, futureTime2)

	// Create a mock server that returns our feed
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprintln(w, feedContent)
	}))
	defer mockServer.Close()

	// Temporarily replace the feeds with our mock server
	originalFeeds := feeds
	defer func() { feeds = originalFeeds }()
	feeds = []string{mockServer.URL}

	// Reset lastCheckedTime to ensure our jobs are "new"
	mu.Lock()
	lastCheckedTime = time.Now().Add(-1 * time.Hour)
	mu.Unlock()

	// Create a recorder to capture the response
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/new", nil)

	// Call the handler
	handleNewJobs(recorder, req)
	
	assert.True(t, emailSent)
	assert.Contains(t, emailSubject, "New Job Postings")
	assert.Contains(t, emailBody, "Senior Golang Developer")
	assert.Contains(t, emailBody, "Golang Engineer")
}

// Manual test function that can be run directly
func TestManualEmailSend(t *testing.T) {
	// Create two test job items with future dates
	job1 := &gofeed.Item{
		Title:     "Senior Golang Developer at Amazing Company",
		Link:      "https://example.com/job/1",
		Published: time.Now().Format(time.RFC1123Z),
		PublishedParsed: func() *time.Time {
			t := time.Now().Add(1 * time.Hour)
			return &t
		}(),
	}

	job2 := &gofeed.Item{
		Title:     "Golang Engineer for Startup",
		Link:      "https://example.com/job/2",
		Published: time.Now().Format(time.RFC1123Z),
		PublishedParsed: func() *time.Time {
			t := time.Now().Add(2 * time.Hour)
			return &t
		}(),
	}

	// Prepare the jobs
	jobs := []*gofeed.Item{job1, job2}
	jobsText := formatJobs(jobs)

	// Send the email
	err := sendEmailNotification(fmt.Sprintf("Test Email - New Job Postings (%d)", len(jobs)), jobsText)
	assert.NoError(t, err)

	fmt.Println("Manual test email sent. Check your inbox.")
}
