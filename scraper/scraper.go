// vim: ts=4:sw=4
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"issues/jira"
	"os"
	"time"
)

func main() {
	var baseURL string
	var debug bool
	var N int
	var outputFile string
	flag.StringVar(&baseURL, "url", "https://issues.apache.org/jira", "base JIRA url")
	flag.IntVar(&N, "n", 1, "concurrent fetches")
	flag.StringVar(&outputFile, "output", "apache.json", "output file for database")
	flag.BoolVar(&debug, "debug", true, "debug output")
	flag.Parse()

	startTime := time.Now()

	// Get the database
	t := jira.NewTracker(baseURL)
	t.FetchAll(N)
	db := t.GetAll()

	// Print out some statistics
	if debug {
		fmt.Printf("%d issues, %d parent links, %d general links\n",
			len(db.Issues),
			len(db.Tree),
			len(db.Graph))
	}

	// Output database
	f, err := os.Create(outputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not output json!")
		os.Exit(1)
	}
	enc := json.NewEncoder(f)
	enc.Encode(db)
	f.Close()
	fmt.Println("run took ", time.Since(startTime))
}
