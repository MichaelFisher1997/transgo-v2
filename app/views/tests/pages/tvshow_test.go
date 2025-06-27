package pages_test

import (
	"testing"
	"transogov2/app/models"
	"transogov2/app/views/pages"
	"transogov2/app/views/tests/testutils"

	"github.com/stretchr/testify/assert"
)

func TestTVShowComponent(t *testing.T) {
	tests := []struct {
		name     string
		tvshow   models.TVShow
		seasons  []models.Season
		contains []string
	}{
		{
			name:     "complete data",
			tvshow:   testutils.MockTVShow(),
			seasons:  testutils.MockSeasons(1, 3),
			contains: []string{"Test Show", "8.5", "Season 1"},
		},
		{
			name:     "missing poster",
			tvshow:   testutils.MockTVShowEmpty(),
			seasons:  testutils.MockSeasons(2, 1),
			contains: []string{"Empty Show", "placeholder.png"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure required fields are initialized
			if tt.tvshow.ID == 0 {
				tt.tvshow.ID = 1
			}
			if tt.tvshow.Title == "" {
				tt.tvshow.Title = "Default Title"
			}
			rendered := testutils.MustRender(pages.TVShow(tt.tvshow, tt.seasons))
			for _, s := range tt.contains {
				assert.Contains(t, rendered, s)
			}
		})
	}
}
