package helpers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

/*
This can be used to import from a "HAR" file which is what the Chrome dev tools exports
its data as. Using this it gives us teh abilty to replay requests as seen by the browser.
*/

var (
	prefixStrip = regexp.MustCompile("^http[s]?:\\/\\/")
	apiMatch    = regexp.MustCompile("\\/api\\/")
)

type Query struct {
	Name   string `yaml:"name"`
	Method string `yaml:"method"`
	Query  string `yaml:"query"`

	// TODO add
	//Disabled     bool
	//InternalAddr string
}

type Queries []Query

type Inputs struct {
	Queries Queries `yaml:"queries"`
}

func (i Inputs) GetRandomQuery() Query {
	return i.Queries[rand.Intn(len(i.Queries))]
}

func LoadInputs(path string) (Inputs, error) {
	var inputs Inputs

	b, err := os.ReadFile(path)
	if err != nil {
		return inputs, fmt.Errorf("failed to read query file: %w", err)
	}

	if err := yaml.Unmarshal(b, &inputs); err != nil {
		return inputs, err
	}

	if len(inputs.Queries) == 0 {
		return inputs, fmt.Errorf("no queries defined in file %s", path)
	}

	return inputs, nil
}

type HarRequest struct {
	Method string
	URL    string
}

type HarResponse struct {
	Status int
}

func (r HarRequest) StrippedURL() string {
	url := prefixStrip.ReplaceAllString(r.URL, "")
	return url[strings.Index(url, "/"):]
}

type Entry struct {
	ServerIPAddress string
	StartedDateTime string
	Request         HarRequest
	Response        HarResponse
}

func (e Entry) canAdd() bool {
	if e.Request.Method != http.MethodGet {
		return false
	}

	if e.Response.Status != 200 {
		return false
	}

	if !apiMatch.MatchString(e.Request.URL) {
		return false
	}

	return true
}

type Log struct {
	Entries []Entry
}

type HAR struct {
	Log Log
}

func ParseAndSaveHarFile(sources []string, dst string) error {
	slog.Debug("ParseAndSaveHarFile", "sources", sources, "dst", dst)

	inputs, err := ParseHARFile(sources)
	if err != nil {
		return err
	}

	b, err := yaml.Marshal(&inputs)
	if err != nil {
		return err
	}

	if err := os.WriteFile(dst, b, 0644); err != nil {
		return err
	}

	return nil
}

func ParseHARFile(paths []string) (Inputs, error) {
	var har HAR
	var index int

	urls := make(map[string]int)

	inputs := Inputs{
		Queries: make(Queries, 0, 100),
	}

	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			return inputs, err
		}

		if err := json.Unmarshal(b, &har); err != nil {
			return inputs, err
		}

		entries := har.Log.Entries

		slog.Debug("ParseHARFile", "num entries", len(entries))

		for _, e := range entries {
			_, exists := urls[e.Request.URL]

			if !exists && e.canAdd() {
				inputs.Queries = append(inputs.Queries, Query{Name: fmt.Sprintf("query %d", index), Method: e.Request.Method, Query: e.Request.StrippedURL()})
				index++
			}

			if !exists {
				urls[e.Request.URL] = 1
			} else {
				urls[e.Request.URL] += 1
			}
		}
	}

	slog.Debug("ParseHARFile", "num usable entries", index)

	return inputs, nil
}
