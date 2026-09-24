package main

import (
	"flag"
	"log"
	"strings"
)

var taskFlag = flag.String("task", "data", "Task to run: data (generate data.go), headers/csp (sync CSP headers), dist (assemble dist/), readme (update README.md wasm sizes), assets (sync .assets/ into index.html), all (run all tasks)")

func main() {
	flag.Parse()
	task := strings.ToLower(strings.TrimSpace(*taskFlag))

	cfg := LoadConfigFromEnv()

	allTasks := []struct {
		name string
		fn   func() error
	}{
		{"assets", syncAssets},
		{"data", func() error { return generateData(cfg) }},
		{"headers", updateCSPHeaders},
		{"dist", assembleDist},
		{"readme", updateReadmeWasmSizes},
	}

	if task == "all" {
		for _, t := range allTasks {
			if err := t.fn(); err != nil {
				log.Fatalf("task %s failed: %v", t.name, err)
			}
		}
		return
	}

	if task == "csp" {
		task = "headers"
	}

	for _, t := range allTasks {
		if t.name == task {
			if err := t.fn(); err != nil {
				log.Fatalf("task %s failed: %v", t.name, err)
			}
			return
		}
	}

	log.Fatalf("unknown task %q. Valid options: data, headers, csp, dist, readme, assets, all", task)
}
