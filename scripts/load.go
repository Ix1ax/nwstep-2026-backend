package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type result struct {
	statusCode int
	duration   time.Duration
	err        error
}

func main() {
	targetURL := flag.String("url", "http://localhost:8080/health", "Target URL to benchmark")
	concurrency := flag.Int("c", 20, "Number of concurrent workers")
	duration := flag.Duration("d", 30*time.Second, "Test duration (e.g. 10s, 30s, 1m)")
	rateLimit := flag.Int("rate", 0, "Target total RPS limit (0 = max throughput)")
	timeout := flag.Duration("timeout", 5*time.Second, "HTTP request timeout")
	flag.Parse()

	fmt.Println("==================================================================")
	fmt.Println("  🚀 XenoChoice API Load Tester (RPS & Latency Benchmark)")
	fmt.Println("==================================================================")
	fmt.Printf("Target URL:    %s\n", *targetURL)
	fmt.Printf("Concurrency:   %d workers\n", *concurrency)
	fmt.Printf("Duration:      %s\n", *duration)
	if *rateLimit > 0 {
		fmt.Printf("Target Rate:   %d RPS\n", *rateLimit)
	} else {
		fmt.Printf("Target Rate:   Max throughput\n")
	}
	fmt.Printf("Timeout:       %s\n", *timeout)
	fmt.Println("------------------------------------------------------------------")

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	// Handle graceful stop on Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n[!] Received interrupt signal, finishing benchmark...")
		cancel()
	}()

	client := &http.Client{
		Timeout: *timeout,
		Transport: &http.Transport{
			MaxIdleConns:        500,
			MaxIdleConnsPerHost: 200,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression: true,
		},
	}

	resultsChan := make(chan result, 50000)
	var totalRequests uint64
	var totalSuccess uint64
	var totalFailed uint64

	var rateTicker *time.Ticker
	var rateLimiter <-chan time.Time
	if *rateLimit > 0 {
		interval := time.Duration(float64(time.Second) / float64(*rateLimit))
		rateTicker = time.NewTicker(interval)
		defer rateTicker.Stop()
		rateLimiter = rateTicker.C
	}

	startTime := time.Now()
	var wg sync.WaitGroup

	// Workers
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if rateLimiter != nil {
						select {
						case <-ctx.Done():
							return
						case <-rateLimiter:
						}
					}

					reqStart := time.Now()
					req, err := http.NewRequestWithContext(ctx, "GET", *targetURL, nil)
					if err != nil {
						if ctx.Err() != nil {
							return
						}
						atomic.AddUint64(&totalFailed, 1)
						resultsChan <- result{duration: time.Since(reqStart), err: err}
						continue
					}

					resp, err := client.Do(req)
					reqDuration := time.Since(reqStart)

					if err != nil {
						if ctx.Err() != nil {
							return
						}
						atomic.AddUint64(&totalFailed, 1)
						resultsChan <- result{duration: reqDuration, err: err}
						continue
					}

					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()

					atomic.AddUint64(&totalRequests, 1)
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						atomic.AddUint64(&totalSuccess, 1)
					} else {
						atomic.AddUint64(&totalFailed, 1)
					}

					resultsChan <- result{
						statusCode: resp.StatusCode,
						duration:   reqDuration,
					}
				}
			}
		}()
	}

	// Live reporter ticker
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		var lastReqs uint64
		lastTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				currentReqs := atomic.LoadUint64(&totalRequests)
				elapsedSec := t.Sub(lastTime).Seconds()
				instantRPS := float64(currentReqs-lastReqs) / elapsedSec
				lastReqs = currentReqs
				lastTime = t

				fmt.Printf("\r[Running] Elapsed: %-4s | Total: %-7d | Current RPS: %-8.1f | Success: %-6d | Errors: %-5d",
					time.Since(startTime).Truncate(time.Second),
					currentReqs,
					instantRPS,
					atomic.LoadUint64(&totalSuccess),
					atomic.LoadUint64(&totalFailed),
				)
			}
		}
	}()

	// Wait for workers to finish
	wg.Wait()
	actualDuration := time.Since(startTime)
	close(resultsChan)

	// Collect and process results
	var allDurations []time.Duration
	statusCounts := make(map[int]int)
	var errCount int

	for res := range resultsChan {
		if res.err != nil {
			errCount++
			continue
		}
		statusCounts[res.statusCode]++
		allDurations = append(allDurations, res.duration)
	}

	sort.Slice(allDurations, func(i, j int) bool {
		return allDurations[i] < allDurations[j]
	})

	totalCount := len(allDurations) + errCount
	avgRPS := float64(len(allDurations)) / actualDuration.Seconds()

	fmt.Printf("\n\n==================================================================\n")
	fmt.Println("  📊 BENCHMARK SUMMARY REPORT (XenoChoice API)")
	fmt.Println("==================================================================")
	fmt.Printf("Total Duration:        %s\n", actualDuration.Round(time.Millisecond))
	fmt.Printf("Total Requests:        %d\n", totalCount)
	fmt.Printf("Successful Requests:   %d (%.2f%%)\n", len(allDurations), float64(len(allDurations))/float64(totalCount)*100)
	fmt.Printf("Failed / Errors:       %d\n", errCount)
	fmt.Printf("Average Throughput:    \033[1;32m%.2f RPS\033[0m\n", avgRPS)
	fmt.Println("------------------------------------------------------------------")
	fmt.Println("Status Code Breakdown:")
	for code, count := range statusCounts {
		fmt.Printf("  HTTP %d: %d (%.2f%%)\n", code, count, float64(count)/float64(totalCount)*100)
	}
	if errCount > 0 {
		fmt.Printf("  Network Errors: %d\n", errCount)
	}
	fmt.Println("------------------------------------------------------------------")

	if len(allDurations) > 0 {
		var totalDur time.Duration
		for _, d := range allDurations {
			totalDur += d
		}
		avgLatency := totalDur / time.Duration(len(allDurations))

		p50 := allDurations[int(float64(len(allDurations))*0.50)]
		p90 := allDurations[int(float64(len(allDurations))*0.90)]
		p95 := allDurations[int(float64(len(allDurations))*0.95)]
		p99 := allDurations[int(float64(len(allDurations))*0.99)]
		minLatency := allDurations[0]
		maxLatency := allDurations[len(allDurations)-1]

		fmt.Println("Latency Distribution:")
		fmt.Printf("  Min:    %s\n", minLatency.Round(time.Microsecond))
		fmt.Printf("  Avg:    %s\n", avgLatency.Round(time.Microsecond))
		fmt.Printf("  p50:    \033[1;36m%s\033[0m\n", p50.Round(time.Microsecond))
		fmt.Printf("  p90:    %s\n", p90.Round(time.Microsecond))
		fmt.Printf("  p95:    \033[1;33m%s\033[0m\n", p95.Round(time.Microsecond))
		fmt.Printf("  p99:    \033[1;31m%s\033[0m\n", p99.Round(time.Microsecond))
		fmt.Printf("  Max:    %s\n", maxLatency.Round(time.Microsecond))
	}
	fmt.Println("==================================================================")
	fmt.Println("💡 Tip: Check your Grafana dashboard at http://localhost:3000")
	fmt.Println("   Dashboard: 'XenoChoice API · Performance & RPS'")
	fmt.Println("==================================================================")
}
