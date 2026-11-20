package main

import (
	"fmt"
	"strings"
)

func recoverySummaryText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "RECOVERY CONFIRMED BY ORIGINAL VAULT EXECUTABLE\r\n\r\nRestore point: %s\r\n", r.Point)
	for _, it := range r.Items {
		fmt.Fprintf(&b, "Item: %s (%s)\r\nDestination: %s\r\nFiles written: %d\r\nData written: %s (%d bytes)\r\n", it.Item, it.Type, it.Dir, it.Files, readableBytes(it.Bytes), it.Bytes)
		fmt.Fprintf(&b, "Renamed paths: %d\r\nSkipped paths: %d\r\nPruned entries: %d\r\n", len(it.Renamed), len(it.Skipped), it.Pruned)
		if it.Error != "" {
			fmt.Fprintf(&b, "Engine item error: %s\r\n", it.Error)
		}
		if len(it.Renamed) > 0 {
			b.WriteString("\r\nRENAMED BY VAULT\r\n")
			for i, n := range it.Renamed {
				if i == 100 {
					fmt.Fprintf(&b, "... and %d more, see Vault JSON report\r\n", len(it.Renamed)-i)
					break
				}
				fmt.Fprintf(&b, "%s -> %s\r\n", n.From, n.To)
			}
		}
		if len(it.Skipped) > 0 {
			b.WriteString("\r\nSKIPPED BY VAULT\r\n")
			for i, n := range it.Skipped {
				if i == 100 {
					fmt.Fprintf(&b, "... and %d more, see Vault JSON report\r\n", len(it.Skipped)-i)
					break
				}
				fmt.Fprintf(&b, "%s: %s\r\n", n.Path, n.Reason)
			}
		}
	}
	fmt.Fprintf(&b, "\r\nOriginal Vault report: %s\r\n", r.ReportPath)
	b.WriteString("\r\nThis summary is read from the original Vault report, not a second verification engine.\r\n")
	return b.String()
}
