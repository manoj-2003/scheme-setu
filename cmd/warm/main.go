// Command warm pre-fetches SerpApi responses for a set of demo personas into
// the committed fixture cache.
//
// Run this once, with a key, before recording the demo. Afterwards the server
// can run with SERP_MODE=cache and answer every persona instantly, offline,
// for zero credits -- which is what makes the recording reliable and lets
// anyone clone the repo and see it work without signing up for anything.
//
//	go run ./cmd/warm -personas fixtures/personas.json -cache fixtures/serp-cache.sqlite
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/manoj-2003/scheme-setu/internal/catalog"
	"github.com/manoj-2003/scheme-setu/internal/discovery"
	"github.com/manoj-2003/scheme-setu/internal/fraud"
	"github.com/manoj-2003/scheme-setu/internal/freshness"
	"github.com/manoj-2003/scheme-setu/internal/models"
	"github.com/manoj-2003/scheme-setu/internal/rules"
	"github.com/manoj-2003/scheme-setu/internal/serp"
)

type persona struct {
	Label   string         `json:"label"`
	Profile models.Profile `json:"profile"`
}

func main() {
	personasPath := flag.String("personas", "fixtures/personas.json", "persona file to warm")
	cachePath := flag.String("cache", "fixtures/serp-cache.sqlite", "cache file to fill")
	maxCredits := flag.Int("max-credits", 60, "hard ceiling on SerpApi calls for this run")
	langs := flag.String("langs", "en,hi", "comma-separated hl values")
	dryRun := flag.Bool("dry-run", false, "print the queries that would be issued and exit")
	flag.Parse()

	if err := run(*personasPath, *cachePath, *langs, *maxCredits, *dryRun); err != nil {
		log.Fatalf("warm: %v", err)
	}
}

func run(personasPath, cachePath, langs string, maxCredits int, dryRun bool) error {
	_ = godotenv.Load()

	personas, err := loadPersonas(personasPath)
	if err != nil {
		return err
	}

	cat, err := catalog.Load()
	if err != nil {
		return err
	}

	languages := splitCSV(langs)

	if dryRun {
		return printPlan(personas, languages)
	}

	if os.Getenv("SERPAPI_KEY") == "" {
		return fmt.Errorf("SERPAPI_KEY is not set, so there is nothing to warm with")
	}

	client, err := serp.New(serp.Options{
		APIKey:     os.Getenv("SERPAPI_KEY"),
		Mode:       serp.ModeLive,
		CachePath:  cachePath,
		CacheTTL:   0, // fixtures should never expire
		MaxCredits: maxCredits,
	})
	if err != nil {
		return err
	}
	defer client.Close()

	fresh := freshness.New(client)
	shield := fraud.New(client, cat.OfficialDomains())

	for _, p := range personas {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)

		eligible, _ := rules.EvaluateAll(p.Profile, cat.Candidates(p.Profile))

		res := discovery.Run(ctx, client, p.Profile, discovery.Options{
			Languages:     languages,
			KnownKeywords: cat.Keywords(),
		})
		for _, w := range res.Warnings {
			log.Printf("  discovery: %s", w)
		}

		for _, w := range fresh.CheckAll(ctx, eligible) {
			log.Printf("  freshness: %s", w)
		}

		for i := range eligible {
			if i >= 3 {
				break
			}
			if _, err := shield.Check(ctx, eligible[i].Scheme); err != nil {
				log.Printf("  fraud: %s: %v", eligible[i].Scheme.ID, err)
			}
		}

		calls, hits, live, _ := client.Usage()
		log.Printf("%-42s matched=%-2d leads=%-2d | calls=%d cached=%d live=%d",
			p.Label, len(eligible), len(res.Leads), calls, hits, live)

		cancel()
	}

	calls, hits, live, _ := client.Usage()
	log.Printf("done: %d searches (%d served from cache, %d live credits spent)", calls, hits, live)
	log.Printf("cache now holds %d responses at %s", client.CachedResponses(context.Background()), cachePath)
	log.Println("commit the cache file, then run the server with SERP_MODE=cache for the demo")
	return nil
}

func printPlan(personas []persona, languages []string) error {
	total := 0
	for _, p := range personas {
		fmt.Printf("\n%s\n", p.Label)
		for _, q := range discovery.Plan(p.Profile, languages, 6) {
			fmt.Printf("  [%-20s %s] %s\n", q.Label, q.Language, q.Params.Query())
			total++
		}
	}
	fmt.Printf("\n%d discovery queries across %d personas (freshness and fraud checks add more)\n",
		total, len(personas))
	return nil
}

func loadPersonas(path string) ([]persona, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read personas: %w", err)
	}
	var personas []persona
	if err := json.Unmarshal(b, &personas); err != nil {
		return nil, fmt.Errorf("parse personas: %w", err)
	}
	if len(personas) == 0 {
		return nil, fmt.Errorf("no personas in %s", path)
	}
	return personas, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range splitComma(s) {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, trim(cur))
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, trim(cur))
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
