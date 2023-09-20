package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/goccy/go-json"

	"anime-vod-data/external"
	_ "anime-vod-data/logger"
)

func main() {
	ctx := context.Background()
	client := external.NewAnnictClient(
		http.DefaultClient,
		"anime-vod-data (+https://github.com/SlashNephy/anime-vod-data)",
	)

	newestWorkID, err := client.FetchNewestWorkID(ctx)
	if err != nil {
		panic(err)
	}

	slog.Info("found newest work id", slog.Int("newest_work_id", newestWorkID))

	var results []*external.AnnictVODData
	workID := 1
	for workID <= newestWorkID {
		slog.Info("fetching vod data", slog.Int("work_id", workID))

		data, err := client.FetchVODData(ctx, workID)
		if err != nil {
			if errors.Is(err, external.ErrRateLimited) {
				slog.Warn("rate limited", slog.Int("work_id", workID))
				time.Sleep(3 * time.Second)
				continue
			}

			panic(err)
		}

		workID++
		results = append(results, data...)
	}

	content, err := json.MarshalContext(ctx, results)
	if err != nil {
		panic(err)
	}

	if err = os.WriteFile("dist/data.json", content, 0644); err != nil {
		panic(err)
	}
}
