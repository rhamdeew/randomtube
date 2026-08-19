package youtube_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"randomtube/internal/youtube"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		input       string
		wantType    youtube.SourceType
		wantID      string
		wantVideoID string
		wantErr     bool
	}{
		{
			input:    "https://www.youtube.com/playlist?list=PLxxx123",
			wantType: youtube.SourcePlaylist, wantID: "PLxxx123",
		},
		{
			input:    "https://www.youtube.com/watch?v=abc&list=PLyyy456",
			wantType: youtube.SourcePlaylist, wantID: "PLyyy456",
			wantVideoID: "abc",
		},
		{
			input:    "https://www.youtube.com/channel/UCabc123",
			wantType: youtube.SourceChannel, wantID: "UCabc123",
		},
		{
			input:    "https://www.youtube.com/@mychannel",
			wantType: youtube.SourceChannel, wantID: "@mychannel",
		},
		{
			input:    "https://www.youtube.com/user/someuser",
			wantType: youtube.SourceChannel, wantID: "someuser",
		},
		{
			input:    "UCabc123",
			wantType: youtube.SourceChannel, wantID: "UCabc123",
		},
		{
			input:    "PLabc123",
			wantType: youtube.SourcePlaylist, wantID: "PLabc123",
		},
		{
			input:    "UUabc123",
			wantType: youtube.SourcePlaylist, wantID: "UUabc123",
		},
		{
			input:   "https://example.com/not-youtube",
			wantErr: true,
		},
		{
			input:   "just-some-text",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			src, err := youtube.ParseURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got src=%+v", src)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if src.Type != tt.wantType {
				t.Errorf("type: got %v, want %v", src.Type, tt.wantType)
			}
			if src.ID != tt.wantID {
				t.Errorf("id: got %q, want %q", src.ID, tt.wantID)
			}
			if src.VideoID != tt.wantVideoID {
				t.Errorf("videoID: got %q, want %q", src.VideoID, tt.wantVideoID)
			}
		})
	}
}

func TestFetchAll_Playlist_IncludesSeedVideo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/playlistItems"):
			_, _ = w.Write([]byte(`{"items":[
				{"snippet":{"title":"Playlist Video","resourceId":{"videoId":"plvid1"}}}
			]}`))
		case strings.Contains(r.URL.Path, "/videos"):
			_, _ = w.Write([]byte(`{"items":[{"id":"seedvid1","snippet":{"title":"Seed Video"}}]}`))
		}
	}))
	defer server.Close()

	f := youtube.New("test-key")
	f.BaseURL = server.URL

	src, err := youtube.ParseURL("https://www.youtube.com/watch?v=seedvid1&list=PLxxx123")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	var got []youtube.VideoItem
	total, err := f.FetchAll(context.Background(), src, func(batch []youtube.VideoItem) {
		got = append(got, batch...)
	})
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 videos, got %d", total)
	}

	ids := map[string]string{}
	for _, item := range got {
		ids[item.YoutubeID] = item.Title
	}
	if ids["plvid1"] != "Playlist Video" {
		t.Errorf("missing playlist video, got %+v", got)
	}
	if ids["seedvid1"] != "Seed Video" {
		t.Errorf("missing seed video, got %+v", got)
	}
}

func TestFetchVideosInfo_ExistingAndMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"exists1","snippet":{"title":"Existing Video"}}]}`))
	}))
	defer server.Close()

	f := youtube.New("test-key")
	f.BaseURL = server.URL

	titles, err := f.FetchVideosInfo(context.Background(), []string{"exists1", "missing1"})
	if err != nil {
		t.Fatalf("FetchVideosInfo: %v", err)
	}
	if got, want := titles["exists1"], "Existing Video"; got != want {
		t.Errorf("titles[exists1] = %q, want %q", got, want)
	}
	if _, ok := titles["missing1"]; ok {
		t.Error("expected missing1 to be absent from titles")
	}
}
