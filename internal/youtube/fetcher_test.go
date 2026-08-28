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
			titles := map[string]string{"seedvid1": "Seed Video", "plvid1": "Playlist Video"}
			var items []string
			for _, id := range strings.Split(r.URL.Query().Get("id"), ",") {
				if title, ok := titles[id]; ok {
					items = append(items, `{"id":"`+id+`","snippet":{"title":"`+title+`"}}`)
				}
			}
			_, _ = w.Write([]byte(`{"items":[` + strings.Join(items, ",") + `]}`))
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

func TestFetchAll_Playlist_SkipsUnavailableVideos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/playlistItems"):
			_, _ = w.Write([]byte(`{"items":[
				{"snippet":{"title":"Alive Video","resourceId":{"videoId":"alive1"}}},
				{"snippet":{"title":"Deleted video","resourceId":{"videoId":"gone1"}}}
			]}`))
		case strings.Contains(r.URL.Path, "/videos"):
			// Only "alive1" actually exists on YouTube; "gone1" was removed
			// (copyright takedown, privacy, etc.) but playlistItems still lists it.
			_, _ = w.Write([]byte(`{"items":[{"id":"alive1","snippet":{"title":"Alive Video"}}]}`))
		}
	}))
	defer server.Close()

	f := youtube.New("test-key")
	f.BaseURL = server.URL

	src, err := youtube.ParseURL("https://www.youtube.com/playlist?list=PLxxx123")
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
	if total != 1 {
		t.Fatalf("expected 1 video (unavailable one filtered out), got %d: %+v", total, got)
	}
	if len(got) != 1 || got[0].YoutubeID != "alive1" {
		t.Errorf("expected only alive1, got %+v", got)
	}
}

const playlistNotFoundBody = `{"error":{"code":404,"message":"The playlist identified with the request's <code>playlistId</code> parameter cannot be found.","errors":[{"message":"The playlist identified with the request's <code>playlistId</code> parameter cannot be found.","domain":"youtube.playlistItem","reason":"playlistNotFound","location":"playlistId","locationType":"parameter"}]}}`

func TestFetchAll_PrivatePlaylistWithSeedVideo_FallsBackToSeedVideo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/playlistItems"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(playlistNotFoundBody))
		case strings.Contains(r.URL.Path, "/videos"):
			_, _ = w.Write([]byte(`{"items":[{"id":"6p6PcFFUm5I","snippet":{"title":"Seed Video"}}]}`))
		}
	}))
	defer server.Close()

	f := youtube.New("test-key")
	f.BaseURL = server.URL

	// list=LL is YouTube's own "Liked videos" playlist — never resolvable via
	// API key, regardless of whose it is.
	src, err := youtube.ParseURL("https://www.youtube.com/watch?v=6p6PcFFUm5I&list=LL&index=4")
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
	if total != 1 || len(got) != 1 || got[0].YoutubeID != "6p6PcFFUm5I" {
		t.Fatalf("expected fallback to the single seed video, got total=%d items=%+v", total, got)
	}
}

func TestFetchAll_DeletedPlaylistWithoutSeedVideo_ReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(playlistNotFoundBody))
	}))
	defer server.Close()

	f := youtube.New("test-key")
	f.BaseURL = server.URL

	// A bare playlist link (no accompanying v=) has nothing to fall back to,
	// so a genuinely missing playlist must still surface as an error.
	src, err := youtube.ParseURL("https://www.youtube.com/playlist?list=PLdeaddeaddead")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	_, err = f.FetchAll(context.Background(), src, func([]youtube.VideoItem) {})
	if err == nil {
		t.Fatal("expected an error for a genuinely missing playlist, got nil")
	}
}

func TestFetchAll_OtherPlaylistErrorWithSeedVideo_StillReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/playlistItems") {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"code":403,"message":"quotaExceeded"}}`))
		}
	}))
	defer server.Close()

	f := youtube.New("test-key")
	f.BaseURL = server.URL

	// Only the specific "playlist doesn't exist" case should fall back to the
	// seed video — a real failure (quota, auth, ...) must not be swallowed.
	src, err := youtube.ParseURL("https://www.youtube.com/watch?v=abc123&list=PLxxx123")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}

	_, err = f.FetchAll(context.Background(), src, func([]youtube.VideoItem) {})
	if err == nil {
		t.Fatal("expected the quota error to propagate, got nil")
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
