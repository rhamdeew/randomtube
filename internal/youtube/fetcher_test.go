package youtube_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"randomtube/internal/youtube"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		input    string
		wantType youtube.SourceType
		wantID   string
		wantErr  bool
	}{
		{
			"https://www.youtube.com/playlist?list=PLxxx123",
			youtube.SourcePlaylist, "PLxxx123", false,
		},
		{
			"https://www.youtube.com/watch?v=abc&list=PLyyy456",
			youtube.SourcePlaylist, "PLyyy456", false,
		},
		{
			"https://www.youtube.com/channel/UCabc123",
			youtube.SourceChannel, "UCabc123", false,
		},
		{
			"https://www.youtube.com/@mychannel",
			youtube.SourceChannel, "@mychannel", false,
		},
		{
			"https://www.youtube.com/user/someuser",
			youtube.SourceChannel, "someuser", false,
		},
		{
			"UCabc123",
			youtube.SourceChannel, "UCabc123", false,
		},
		{
			"PLabc123",
			youtube.SourcePlaylist, "PLabc123", false,
		},
		{
			"UUabc123",
			youtube.SourcePlaylist, "UUabc123", false,
		},
		{
			"https://example.com/not-youtube",
			0, "", true,
		},
		{
			"just-some-text",
			0, "", true,
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
		})
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
