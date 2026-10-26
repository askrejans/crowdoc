package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/askrejans/crowdoc/v2"
)

// runWatch regenerates the PDF whenever the input (or a bibliography file)
// changes. The compilation directory is reused, so rebuilds are fast.
func runWatch(ctx context.Context, c *cli, opts crowdoc.Options, stdout, stderr io.Writer) error {
	if opts.WorkDir == "" {
		dir, err := os.MkdirTemp("", "crowdoc-watch-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		opts.WorkDir = dir
	}
	watched := append([]string{c.input}, c.bibs...)
	stamp := func() string {
		var sb strings.Builder
		for _, p := range watched {
			if fi, err := os.Stat(p); err == nil {
				fmt.Fprintf(&sb, "%s|%d|%d;", p, fi.ModTime().UnixNano(), fi.Size())
			}
		}
		return sb.String()
	}
	fmt.Fprintf(stdout, "Watching %s (Ctrl+C to stop)\n", c.input)
	last := ""
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		if s := stamp(); s != last {
			last = s
			if err := convertOne(ctx, c, c.input, c.output, opts, stdout, stderr); err != nil {
				fmt.Fprintln(stderr, "crowdoc:", err)
			} else if !c.quiet {
				fmt.Fprintf(stdout, "  updated at %s\n", time.Now().Format("15:04:05"))
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
