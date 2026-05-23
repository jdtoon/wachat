// Command bench seeds a wachat SQLite database with N synthetic messages
// and prints the timings that matter for the CLAUDE.md §12 performance
// budget: store open, bulk insert, first-page and deep-page keyset query
// latency, and Go heap size as a proxy for idle memory footprint.
//
// Usage:
//
//	go run ./cmd/bench           # default: 100_000 messages
//	go run ./cmd/bench -n 1000   # smaller seed for quick iteration
//	go run ./cmd/bench -keep     # leave the temp DB on disk for poking
//
// The output is intentionally human-readable rather than machine-parseable
// — these numbers are read by humans deciding whether a change has
// regressed. If we ever want them in CI-friendly form, JSON output goes
// behind a flag.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jdtoon/wachat/internal/store"
)

const benchChatJID = "bench@s.whatsapp.net"

func main() {
	total := flag.Int("n", 100_000, "number of messages to seed")
	pageSize := flag.Int("page", 50, "messages per keyset page")
	keep := flag.Bool("keep", false, "keep the temp DB after the run (path is printed)")
	flag.Parse()

	if err := run(*total, *pageSize, *keep); err != nil {
		log.Fatal(err)
	}
}

func run(total, pageSize int, keep bool) error {
	fmt.Println("wachat bench")
	fmt.Println("============")
	fmt.Printf("messages = %d, page size = %d\n\n", total, pageSize)

	dir, err := os.MkdirTemp("", "wachat-bench-*")
	if err != nil {
		return fmt.Errorf("tempdir: %w", err)
	}
	if !keep {
		defer os.RemoveAll(dir)
	}
	dbPath := filepath.Join(dir, "wachat.db")
	fmt.Println("db path:           ", dbPath)

	ctx := context.Background()

	// --- store open ---
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	openStart := time.Now()
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("store.Open: %w", err)
	}
	defer func() { _ = s.Close() }()
	openDur := time.Since(openStart)
	fmt.Printf("store.Open:         %v\n", openDur)

	// --- bulk insert ---
	seedStart := time.Now()
	for i := 0; i < total; i++ {
		_, err := s.Insert(ctx, store.Message{
			WAID:    fmt.Sprintf("w%08d", i),
			ChatJID: benchChatJID,
			TS:      int64(i + 1),
			Body:    "lorem ipsum dolor sit amet, consectetur adipiscing elit",
		}, false)
		if err != nil {
			return fmt.Errorf("seed insert %d: %w", i, err)
		}
	}
	seedDur := time.Since(seedStart)
	rate := float64(total) / seedDur.Seconds()
	fmt.Printf("seed:               %v  (%.0f msgs/s)\n", seedDur, rate)

	// --- heap snapshot ---
	runtime.GC()
	var memAfterSeed runtime.MemStats
	runtime.ReadMemStats(&memAfterSeed)
	heapMB := float64(memAfterSeed.HeapAlloc) / (1 << 20)
	sysMB := float64(memAfterSeed.Sys) / (1 << 20)
	fmt.Printf("Go heap (HeapAlloc):%.1f MB\n", heapMB)
	fmt.Printf("Go RSS proxy (Sys): %.1f MB\n", sysMB)

	// --- first page (newest end) ---
	firstStart := time.Now()
	_, next, err := s.PageOlder(ctx, benchChatJID, store.Cursor{}, pageSize)
	if err != nil {
		return fmt.Errorf("first page: %w", err)
	}
	firstDur := time.Since(firstStart)
	fmt.Printf("first page:         %v\n", firstDur)

	// --- walk to ~90% then time a page ---
	cursor := next
	walkPages := (total - pageSize) * 9 / 10 / pageSize
	for i := 0; i < walkPages; i++ {
		_, c, err := s.PageOlder(ctx, benchChatJID, cursor, pageSize)
		if err != nil {
			return fmt.Errorf("walk page %d: %w", i, err)
		}
		cursor = c
	}

	deepStart := time.Now()
	_, _, err = s.PageOlder(ctx, benchChatJID, cursor, pageSize)
	if err != nil {
		return fmt.Errorf("deep page: %w", err)
	}
	deepDur := time.Since(deepStart)
	fmt.Printf("deep page (~90%%):   %v\n", deepDur)

	// --- summary line for quick scan ---
	fmt.Println()
	ratio := float64(deepDur) / float64(firstDur)
	fmt.Printf("Summary: deep/first ratio = %.1fx (keyset target: ~1x)\n", ratio)
	fmt.Printf("         total bench time = %v\n", time.Since(openStart))
	if keep {
		fmt.Println("         (run with -keep; db preserved at:", dbPath+")")
	}
	return nil
}
