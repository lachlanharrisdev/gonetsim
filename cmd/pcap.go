package cmd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/lachlanharrisdev/gonetsim/internal/capture"
)

var pcapCmd = &cobra.Command{
	Use:   "pcap <file|dir>",
	Short: "Inspect pcapng capture files",
	Long: "Reads pcapng capture files and prints a summary. Pass a single file\n" +
		"or a directory (e.g. the runs directory) to summarize every capture beneath it.\n" +
		"Legacy pcap files are not supported",
	Args: cobra.ExactArgs(1),
	RunE: runPcap,
}

func init() {
	rootCmd.AddCommand(pcapCmd)
}

func runPcap(cmd *cobra.Command, args []string) error {
	return inspectPcap(cmd.OutOrStdout(), args[0])
}

func inspectPcap(out io.Writer, target string) error {
	st, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("pcap %q: %w", target, err)
	}
	if st.IsDir() {
		return inspectPcapDir(out, target)
	}
	info, err := capture.Inspect(target)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s\n", summarizePcap(target, info))
	return nil
}

func summarizePcap(path string, info capture.FileInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: format=pcapng linktype=%s packets=%d", path, info.LinkType, info.Packets)
	if info.Packets > 0 {
		fmt.Fprintf(&sb, " first=%s last=%s duration=%s",
			info.First.Format(time.RFC3339), info.Last.Format(time.RFC3339),
			info.Last.Sub(info.First).Round(time.Millisecond))
	}
	if len(info.Interfaces) > 0 {
		fmt.Fprintf(&sb, " interfaces=%s", strings.Join(info.Interfaces, "|"))
	}
	if info.CreatedBy != "" {
		fmt.Fprintf(&sb, " app=%s", info.CreatedBy)
	}
	return sb.String()
}

func inspectPcapDir(out io.Writer, dir string) error {
	var files []string
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".pcapng") {
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("pcap %q: %w", dir, walkErr)
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no pcapng files found in %q", dir)
	}
	var total uint64
	failed := 0
	for _, f := range files {
		info, err := capture.Inspect(f)
		if err != nil {
			fmt.Fprintf(out, "%s: ERROR %v\n", f, err)
			failed++
			continue
		}
		fmt.Fprintf(out, "%s\n", summarizePcap(f, info))
		total += info.Packets
	}
	fmt.Fprintf(out, "total: files=%d packets=%d\n", len(files), total)
	if failed > 0 {
		return fmt.Errorf("%d of %d files could not be read", failed, len(files))
	}
	return nil
}
