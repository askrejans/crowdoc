package main

import (
	"fmt"
	"os"
	"time"
)

// watchAndConvert watches the input file for changes and regenerates the PDF.
// Uses polling for simplicity (no external dependencies).
func watchAndConvert(opts options) {
	fmt.Printf("crowdoc: watching %s for changes (Ctrl+C to stop)\n", opts.inputPath)

	// Initial conversion
	if err := convertFile(opts); err != nil {
		fmt.Fprintf(os.Stderr, "crowdoc: %v\n", err)
	}

	var lastModTime time.Time
	info, err := os.Stat(opts.inputPath)
	if err == nil {
		lastModTime = info.ModTime()
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		info, err := os.Stat(opts.inputPath)
		if err != nil {
			continue
		}

		if info.ModTime().After(lastModTime) {
			lastModTime = info.ModTime()
			fmt.Printf("\ncrowdoc: change detected, regenerating...\n")
			if err := convertFile(opts); err != nil {
				fmt.Fprintf(os.Stderr, "crowdoc: %v\n", err)
			} else {
				fmt.Printf("crowdoc: done at %s\n", time.Now().Format("15:04:05"))
			}
		}
	}
}
