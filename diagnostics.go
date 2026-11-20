package main

// Diagnostics are GUI-only observations of the original CLI subprocess.
// They never inspect backup contents beyond the JSON that the CLI returned,
// and never guess how much of a long-running operation is complete.

import (
	"fmt"
	"strings"
	"time"
)

func diagLine(at time.Time, phase, message string) string {
	return fmt.Sprintf("[%s] %-8s %s\r\n", at.Format("15:04:05"), strings.ToUpper(phase), message)
}

func elapsedLabel(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	minutes := int(d / time.Minute)
	seconds := int(d % time.Minute / time.Second)
	if minutes >= 60 {
		return fmt.Sprintf("%dh %02dm %02ds", minutes/60, minutes%60, seconds)
	}
	return fmt.Sprintf("%dm %02ds", minutes, seconds)
}

func byteLabel(n int64) string {
	if n < 0 {
		return "unknown"
	}
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	v := float64(n)
	for _, unit := range []string{"KiB", "MiB", "GiB", "TiB", "PiB"} {
		v /= 1024
		if v < 1024 || unit == "PiB" {
			return fmt.Sprintf("%.1f %s", v, unit)
		}
	}
	return fmt.Sprintf("%d B", n)
}

func pointSummary(points []Point) string {
	locked, items := 0, 0
	for _, p := range points {
		if p.Locked {
			locked++
		}
		items += len(p.Items)
	}
	return fmt.Sprintf("%d restore points, %d listed items, %d locked", len(points), items, locked)
}

func fileSummary(entries []Entry) string {
	files, dirs := 0, 0
	var bytes int64
	for _, e := range entries {
		if e.IsDir {
			dirs++
			continue
		}
		files++
		if e.Size > 0 {
			if bytes > (1<<63-1)-e.Size {
				bytes = 1<<63 - 1
			} else {
				bytes += e.Size
			}
		}
	}
	return fmt.Sprintf("%d paths (%d %s, %d %s), %s indexed file sizes", len(entries), files, noun(files, "file"), dirs, noun(dirs, "folder"), byteLabel(bytes))
}

func noun(n int, singular string) string {
	if n == 1 {
		return singular
	}
	return singular + "s"
}
