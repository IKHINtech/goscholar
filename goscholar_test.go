package goscholar

import (
	"fmt"
	"log"
	"testing"
)

func TestCrawlGoogleScholarByUserID(t *testing.T) {
	StartProxyUpdater()
	// Test the CrawlGoogleScholar function
	results, err := CrawlGoogleScholarByUserID("9vs_d-0AAAAJ")
	if err != nil {
		log.Fatal(err)
	}

	for _, result := range results {
		fmt.Printf("Title: %s\nAuthors: %s\n", result.Title, result.Authors)
	}
}
