// Package main is the entry point for the Spider crawler.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spider-crawler/spider/internal/config"
	"github.com/spider-crawler/spider/internal/frontier"
	"github.com/spider-crawler/spider/internal/scheduler"
)

func main() {
	// Create configuration
	cfg := config.DefaultConfig()
	cfg.Concurrency = 3
	cfg.MaxDepth = 2
	cfg.MaxURLs = 100
	cfg.RequestsPerSecond = 5
	cfg.CrawlDelay = 500 * time.Millisecond
	cfg.Timeout = 10 * time.Second

	// Example seed URL (replace with actual URL to test)
	if len(os.Args) < 2 {
		fmt.Println("Usage: spider <url>")
		fmt.Println("Example: spider https://example.com")
		os.Exit(1)
	}
	seedURL := os.Args[1]

	// Create scheduler
	sched := scheduler.NewScheduler(cfg)

	// Set worker function (placeholder - will be replaced with actual HTTP fetcher)
	sched.SetWorkerFunc(placeholderWorker)

	// Add seed URL
	if err := sched.AddSeed(seedURL); err != nil {
		log.Fatalf("Failed to add seed URL: %v", err)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt signal, stopping...")
		cancel()
		sched.Stop()
	}()

	// Start crawling
	fmt.Printf("Starting crawl with configuration:\n")
	fmt.Printf("  - Concurrency: %d\n", cfg.Concurrency)
	fmt.Printf("  - Max Depth: %d\n", cfg.MaxDepth)
	fmt.Printf("  - Max URLs: %d\n", cfg.MaxURLs)
	fmt.Printf("  - Requests/sec: %.1f\n", cfg.RequestsPerSecond)
	fmt.Printf("  - Crawl Delay: %v\n", cfg.CrawlDelay)
	fmt.Printf("  - Traversal Mode: %s\n", cfg.TraversalMode)
	fmt.Println()

	if err := sched.Start(ctx); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	// Process results in separate goroutine
	go func() {
		for result := range sched.Results() {
			printResult(result)
		}
	}()

	// Print stats periodically
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := sched.Stats()
				fmt.Printf("\n[Stats] Processed: %d | Succeeded: %d | Failed: %d | Queue: %d | Visited: %d | Elapsed: %v\n",
					stats.URLsProcessed, stats.URLsSucceeded, stats.URLsFailed,
					stats.URLsInQueue, stats.URLsVisited, stats.ElapsedTime.Round(time.Second))
			}
		}
	}()

	// Wait for completion
	sched.Wait()

	// Print final stats
	stats := sched.Stats()
	fmt.Println("\n========== Crawl Complete ==========")
	fmt.Printf("Total URLs Processed: %d\n", stats.URLsProcessed)
	fmt.Printf("Succeeded: %d\n", stats.URLsSucceeded)
	fmt.Printf("Failed: %d\n", stats.URLsFailed)
	fmt.Printf("Retried: %d\n", stats.URLsRetried)
	fmt.Printf("Duplicates Skipped: %d\n", stats.TotalDuplicates)
	fmt.Printf("Total Time: %v\n", stats.ElapsedTime.Round(time.Millisecond))
}

// placeholderWorker is a placeholder worker function.
// This will be replaced with actual HTTP fetching logic in the next phase.
func placeholderWorker(ctx context.Context, item *frontier.URLItem) (*scheduler.CrawlResult, error) {
	start := time.Now()

	// Simulate network request
	time.Sleep(50 * time.Millisecond)

	// For now, just return a dummy result
	// The actual implementation will fetch the URL and parse links
	result := &scheduler.CrawlResult{
		Item:          item,
		StatusCode:    200,
		ContentType:   "text/html",
		ContentLength: 1024,
		ResponseTime:  time.Since(start),
		FinalURL:      item.URL,
		// In real implementation, this will be populated by parsing the HTML
		DiscoveredURLs: []string{},
	}

	return result, nil
}

func printResult(result *scheduler.CrawlResult) {
	status := "OK"
	if result.Error != nil {
		status = fmt.Sprintf("ERROR: %v", result.Error)
	}

	fmt.Printf("[%d] %s (depth=%d, time=%v) - %s\n",
		result.StatusCode,
		result.Item.URL,
		result.Item.Depth,
		result.ResponseTime.Round(time.Millisecond),
		status)
}
