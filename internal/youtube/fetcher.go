package youtube

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	dbpkg "randomtube/internal/db"
)

type VideoItem struct {
	YoutubeID string
	Title     string
}

type Fetcher struct {
	apiKey string
	client *http.Client
}

func New(apiKey string) *Fetcher {
	return &Fetcher{apiKey: apiKey, client: &http.Client{}}
}

type SourceType int

const (
	SourcePlaylist SourceType = iota
	SourceChannel
)

type Source struct {
	Type SourceType
	ID   string
}

var (
	rePlaylistID = regexp.MustCompile(`[?&]list=([A-Za-z0-9_-]+)`)
	reChannelID  = regexp.MustCompile(`/channel/([A-Za-z0-9_-]+)`)
	reHandle     = regexp.MustCompile(`/@([A-Za-z0-9_.-]+)`)
	reUserPath   = regexp.MustCompile(`/user/([A-Za-z0-9_.-]+)`)
)

func ParseURL(rawURL string) (*Source, error) {
	if m := rePlaylistID.FindStringSubmatch(rawURL); m != nil {
		return &Source{Type: SourcePlaylist, ID: m[1]}, nil
	}
	if m := reChannelID.FindStringSubmatch(rawURL); m != nil {
		return &Source{Type: SourceChannel, ID: m[1]}, nil
	}
	if m := reHandle.FindStringSubmatch(rawURL); m != nil {
		return &Source{Type: SourceChannel, ID: "@" + m[1]}, nil
	}
	if m := reUserPath.FindStringSubmatch(rawURL); m != nil {
		return &Source{Type: SourceChannel, ID: m[1]}, nil
	}
	// bare channel/handle
	rawURL = strings.TrimSpace(rawURL)
	if strings.HasPrefix(rawURL, "UC") && !strings.Contains(rawURL, "/") {
		return &Source{Type: SourceChannel, ID: rawURL}, nil
	}
	if strings.HasPrefix(rawURL, "PL") || strings.HasPrefix(rawURL, "UU") {
		return &Source{Type: SourcePlaylist, ID: rawURL}, nil
	}
	return nil, fmt.Errorf("cannot parse YouTube URL: %s", rawURL)
}

// maxMixItems caps imports from YouTube "Mix"/radio playlists (IDs prefixed
// with "RD"), which are algorithmically generated on the fly and have no
// fixed length, so the API would otherwise keep paging indefinitely.
const maxMixItems = 500

func (f *Fetcher) FetchAll(ctx context.Context, src *Source, onBatch func([]VideoItem)) (int, error) {
	if src.Type == SourcePlaylist {
		maxItems := 0
		if strings.HasPrefix(src.ID, "RD") {
			maxItems = maxMixItems
		}
		return f.fetchPlaylist(ctx, src.ID, maxItems, onBatch)
	}
	playlistID, err := f.channelUploadsPlaylist(ctx, src.ID)
	if err != nil {
		return 0, err
	}
	return f.fetchPlaylist(ctx, playlistID, 0, onBatch)
}

func (f *Fetcher) channelUploadsPlaylist(ctx context.Context, channelID string) (string, error) {
	params := url.Values{
		"part":   {"contentDetails"},
		"key":    {f.apiKey},
		"fields": {"items/contentDetails/relatedPlaylists/uploads"},
	}

	// handle @handle vs UCxxx
	if strings.HasPrefix(channelID, "@") {
		params.Set("forHandle", channelID[1:])
	} else if strings.HasPrefix(channelID, "UC") {
		params.Set("id", channelID)
	} else {
		params.Set("forUsername", channelID)
	}

	resp, err := f.get(ctx, "https://www.googleapis.com/youtube/v3/channels", params)
	if err != nil {
		return "", err
	}

	var data struct {
		Items []struct {
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resp, &data); err != nil {
		return "", fmt.Errorf("parse channels response: %w", err)
	}
	if len(data.Items) == 0 {
		return "", fmt.Errorf("channel not found: %s", channelID)
	}
	return data.Items[0].ContentDetails.RelatedPlaylists.Uploads, nil
}

func (f *Fetcher) fetchPlaylist(ctx context.Context, playlistID string, maxItems int, onBatch func([]VideoItem)) (int, error) {
	total := 0
	pageToken := ""
	seen := make(map[string]bool)

	for {
		params := url.Values{
			"part":       {"snippet"},
			"playlistId": {playlistID},
			"maxResults": {"50"},
			"key":        {f.apiKey},
			"fields":     {"nextPageToken,items/snippet(title,resourceId/videoId)"},
		}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}

		resp, err := f.get(ctx, "https://www.googleapis.com/youtube/v3/playlistItems", params)
		if err != nil {
			return total, err
		}

		var data struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				Snippet struct {
					Title      string `json:"title"`
					ResourceID struct {
						VideoID string `json:"videoId"`
					} `json:"resourceId"`
				} `json:"snippet"`
			} `json:"items"`
		}
		if err := json.Unmarshal(resp, &data); err != nil {
			return total, fmt.Errorf("parse playlist response: %w", err)
		}

		batch := make([]VideoItem, 0, len(data.Items))
		newInPage := 0
		for _, item := range data.Items {
			vid := item.Snippet.ResourceID.VideoID
			if vid == "" {
				continue
			}
			if !seen[vid] {
				seen[vid] = true
				newInPage++
			}
			batch = append(batch, VideoItem{
				YoutubeID: vid,
				Title:     item.Snippet.Title,
			})
		}
		if len(batch) > 0 {
			onBatch(batch)
			total += len(batch)
		}

		if data.NextPageToken == "" {
			break
		}
		if maxItems > 0 && total >= maxItems {
			break
		}
		// A page with no new video IDs means we've looped back into
		// already-seen content (dynamic/"Mix" playlists never end and
		// cycle through the same videos), so stop paging.
		if len(data.Items) > 0 && newInPage == 0 {
			break
		}
		pageToken = data.NextPageToken

		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}
	}
	return total, nil
}

func (f *Fetcher) get(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube API %d: %s", resp.StatusCode, body)
	}
	return body, nil
}

func RunImport(ctx context.Context, database *sql.DB, fetcher *Fetcher, jobID int64, rawURL string, categoryID *int64) {
	if err := dbpkg.SetImportJobRunning(database, jobID); err != nil {
		return
	}

	src, err := ParseURL(rawURL)
	if err != nil {
		_ = dbpkg.FinishImportJob(database, jobID, err.Error())
		return
	}

	imported := 0
	_, fetchErr := fetcher.FetchAll(ctx, src, func(batch []VideoItem) {
		for _, item := range batch {
			_ = dbpkg.UpsertVideo(database, item.YoutubeID, item.Title, categoryID)
			imported++
		}
		_ = dbpkg.UpdateImportJobProgress(database, jobID, 0, imported)
	})

	errMsg := ""
	if fetchErr != nil {
		errMsg = fetchErr.Error()
	}
	_ = dbpkg.UpdateImportJobProgress(database, jobID, imported, imported)
	_ = dbpkg.FinishImportJob(database, jobID, errMsg)
}
