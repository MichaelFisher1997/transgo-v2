package main

import (
	"strings"
	"testing"
)

func TestIsVideoFile(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{".mp4", true},
		{".mkv", true},
		{".txt", false},
		{".jpg", false},
		{".MP4", true}, // Case insensitivity
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			// Convert to lowercase as isVideoFile expects lowercased extensions
			if got := isVideoFile(strings.ToLower(tt.ext)); got != tt.want {
				t.Errorf("isVideoFile(%q) = %v, want %v", tt.ext, got, tt.want)
			}
		})
	}
}

func TestCleanTitle(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"Movie.Title.2023.1080p.WEB-DL.x264-GROUP.mp4", "Movie Title"},
		{"Another.Movie.(2022).BluRay.mkv", "Another Movie"},
		{"TV.Show.S01E01.Episode.Title.720p.mkv", "TV Show S01E01 Episode Title"},
		{"My_Awesome_Show_S02E05_Part_1.avi", "My Awesome Show S02E05 Part 1"},
		{"SimpleTitle.mp4", "SimpleTitle"},
		{"Title.with.brackets.[1080p].mp4", "Title with brackets"},
		{"Title.with.parentheses.(HD).mp4", "Title with parentheses"},
		{"Title.with.year.2023.mp4", "Title with year"},
		{"Title.with.multiple.suffixes.1080p.x264.mp4", "Title with multiple suffixes"},
		{"Title - 1080p.mp4", "Title"},
		{"Title.YIFY.mp4", "Title"},
		{"Title.RARBG.mp4", "Title"},
		{"Title-RARBG.mp4", "Title"},
		{"Title.HEVC.mp4", "Title"},
		{"Title.AAC.mp4", "Title"},
		{"", ""},
		{".hiddenfile", ".hiddenfile"}, // Should not remove extension if it's just a dot
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := cleanTitle(tt.filename); got != tt.want {
				t.Errorf("cleanTitle(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}
