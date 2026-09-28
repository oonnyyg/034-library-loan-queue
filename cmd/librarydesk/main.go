package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"example.com/library-loan-queue/internal/desk"
)

func main() {
	stateDir := flag.String("state", "", "directory for persistent library data (created if missing)")
	flag.Parse()
	if *stateDir == "" {
		fmt.Fprintln(os.Stderr, "librarydesk: --state is required")
		os.Exit(2)
	}

	d, err := desk.Load(*stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "librarydesk: cannot load state: %v\n", err)
		os.Exit(1)
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	writer := bufio.NewWriter(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		resp := d.Handle(line)
		out, err := json.Marshal(resp)
		if err != nil {
			out = []byte(`{"ok":false,"errorCode":"internal_error","reason":"failed to encode response"}`)
		}
		writer.Write(out)
		writer.WriteByte('\n')
		writer.Flush()

		if resp.OK {
			if saveErr := d.Save(); saveErr != nil {
				fmt.Fprintf(os.Stderr, "librarydesk: cannot persist state: %v\n", saveErr)
				os.Exit(1)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "librarydesk: read error: %v\n", err)
		os.Exit(1)
	}
}
