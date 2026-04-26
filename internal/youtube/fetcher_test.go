package youtube_test

import (
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
