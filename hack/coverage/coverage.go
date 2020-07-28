package main

// This program is used to merge a number of go coverprofiles into a single
// file. The mode is expected to be "count".

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const _mode = "mode: count"

func main() {
	var (
		profile    string
		profileDir string
		sizeFinal  int64
		sizeTotal  int64
	)

	flag.StringVar(&profile, "profile", "", "write results to the specified path")
	flag.StringVar(&profileDir, "dir", "", "coverprofile directory")
	flag.Parse()

	if len(profile) == 0 {
		log.Fatal("empty profile")
	}
	if len(profileDir) == 0 {
		log.Fatal("empty coverprofile directory")
	}

	profiles := []string{}
	err := filepath.Walk(profileDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		sizeTotal += info.Size()
		profiles = append(profiles, path)
		return nil
	})
	if err != nil {
		log.Fatalf("failed reading %q: %s", profileDir, err)
	}

	data := make(map[string]bool)
	for _, profile := range profiles {
		file, err := os.Open(profile)
		if err != nil {
			log.Fatalf("failed opening file: %s", err)
		}

		scanner := bufio.NewScanner(file)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			data[scanner.Text()] = true
		}
	}

	file, err := os.Create(profile)
	if err != nil {
		log.Fatalf("failed opening %q: %s", profile, err)
	}

	writeLine := func(line string) {
		n, err := file.WriteString(line + "\n")
		if err != nil {
			log.Fatalf("failed writing %q to %q: %s", line, profile, err)
		}
		sizeFinal += int64(n)
	}

	// Make sure we have the "mode: count" header.
	delete(data, _mode)
	writeLine(_mode)
	for line := range data {
		writeLine(line)
	}
	fmt.Printf(
		"Merged %d profiles (%s) into %q (%s)\n",
		len(profiles),
		byteCountSI(sizeTotal),
		profile,
		byteCountSI(sizeFinal),
	)
}

// Copied from https://yourbasic.org/golang/formatting-byte-size-to-human-readable-format/
func byteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}
