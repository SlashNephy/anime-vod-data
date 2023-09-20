package external

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type AnnictClient struct {
	client    *http.Client
	userAgent string
}

func NewAnnictClient(client *http.Client, userAgent string) *AnnictClient {
	return &AnnictClient{
		client:    client,
		userAgent: userAgent,
	}
}

func (a *AnnictClient) FetchNewestWorkID(ctx context.Context) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.annict.com/works/newest", nil)
	if err != nil {
		return 0, err
	}

	request.Header.Set("User-Agent", a.userAgent)

	response, err := a.client.Do(request)
	if err != nil {
		return 0, err
	}

	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	defer response.Body.Close()
	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return 0, err
	}

	href, ok := document.Find("div.c-work-card > a").First().Attr("href")
	if !ok {
		return 0, errors.New("no href")
	}

	rawID := strings.TrimPrefix(href, "/works/")
	return strconv.Atoi(rawID)
}

type AnnictVODData struct {
	WorkID        int     `json:"work_id"`
	ProgramID     int     `json:"program_id"`
	ChannelID     int     `json:"channel_id"`
	ChannelName   string  `json:"channel_name"`
	StartedAt     *string `json:"started_at,omitempty"`
	IsRebroadcast bool    `json:"is_rebroadcast"`
	VODCode       *string `json:"vod_code,omitempty"`
	VODTitle      *string `json:"vod_title,omitempty"`
}

var ErrRateLimited = errors.New("rate limited")

func (a *AnnictClient) FetchVODData(ctx context.Context, workID int) ([]*AnnictVODData, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://api.annict.com/db/works/%d/programs", workID), nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("User-Agent", a.userAgent)

	response, err := a.client.Do(request)
	if err != nil {
		return nil, err
	}

	var results []*AnnictVODData
	switch response.StatusCode {
	case http.StatusOK:
		break
	case http.StatusNotFound:
		return results, nil
	case http.StatusTooManyRequests:
		return nil, ErrRateLimited
	default:
		return nil, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	defer response.Body.Close()
	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return nil, err
	}

	nullishString := func(text string) *string {
		t := strings.TrimSpace(text)
		if t == "" || t == "-" {
			return nil
		}

		return &t
	}

	document.Find("table.table tbody tr").Each(func(i int, selection *goquery.Selection) {
		children := selection.Children()
		if children.Length() != 9 {
			slog.Error("invalid table row", slog.Int("row", i), slog.Int("length", children.Length()), slog.Int("work_id", workID))
			return
		}

		programID, err := strconv.Atoi(strings.TrimSpace(children.Eq(0).Text()))
		if err != nil {
			slog.Error("invalid program id", slog.Int("row", i), slog.Int("program_id", programID), slog.Int("work_id", workID))
			return
		}

		channelID, err := strconv.Atoi(strings.TrimSpace(children.Eq(1).Text()))
		if err != nil {
			slog.Error("invalid channel id", slog.Int("row", i), slog.Int("channel_id", channelID), slog.Int("work_id", workID))
			return
		}

		if strings.TrimSpace(children.Eq(7).Text()) != "公開" {
			slog.Error("invalid status", slog.Int("row", i), slog.Int("work_id", workID))
			return
		}

		results = append(results, &AnnictVODData{
			WorkID:        workID,
			ProgramID:     programID,
			ChannelID:     channelID,
			ChannelName:   strings.TrimSpace(children.Eq(2).Text()),
			StartedAt:     nullishString(children.Eq(3).Text()),
			IsRebroadcast: children.Eq(4).HasClass("fa-circle"),
			VODCode:       nullishString(children.Eq(5).Text()),
			VODTitle:      nullishString(children.Eq(6).Text()),
		})
	})

	return results, nil
}
