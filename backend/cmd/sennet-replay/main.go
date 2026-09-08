// Command sennet-replay requests an authenticated, storage-idempotent replay.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/sennet/sennet/backend/platform"
)

func main() {
	endpoint := flag.String("url", "http://127.0.0.1:8080", "query/control base URL")
	token := flag.String("token", os.Getenv("SENNET_API_KEY"), "admin API key")
	id := flag.String("id", "", "dead-letter record ID")
	eventFile := flag.String("event", "", "optional corrected Event JSON file")
	flag.Parse()
	if *token == "" || *id == "" {
		fmt.Fprintln(os.Stderr, "-token and -id are required")
		os.Exit(2)
	}
	request := struct {
		ID    string          `json:"id"`
		Event *platform.Event `json:"event,omitempty"`
	}{ID: *id}
	if *eventFile != "" {
		b, err := os.ReadFile(*eventFile)
		if err != nil {
			fatal(err)
		}
		var e platform.Event
		if err = json.Unmarshal(b, &e); err != nil {
			fatal(fmt.Errorf("event file must contain one valid Event JSON object: %w", err))
		}
		request.Event = &e
	}
	body, _ := json.Marshal(request)
	req, err := http.NewRequest("POST", *endpoint+"/api/dead-letter", bytes.NewReader(body))
	if err != nil {
		fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+*token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusAccepted {
		fatal(fmt.Errorf("replay failed (%d): %s", resp.StatusCode, out))
	}
	_, _ = os.Stdout.Write(out)
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
