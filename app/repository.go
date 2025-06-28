package main

import (
	"context"

	"github.com/MichaelFisher1997/transgo-v2/app/models"
)

// Repository defines the methods for database operations
type MediaRepository interface {
	SaveMedia(ctx context.Context, media *models.Media) (int64, error)
	GetMediaByPath(ctx context.Context, path string) (models.Media, error)
	SaveTVShow(ctx context.Context, tvshow *models.TVShow) (int64, error)
	GetTVShowByPath(ctx context.Context, path string) (models.TVShow, error)
	GetAllTVShows(ctx context.Context) ([]models.TVShow, error)
	GetTVShowByID(ctx context.Context, id int64) (models.TVShow, error)
	GetSeasonsByTVShowID(ctx context.Context, tvshowID int64) ([]models.Season, error)
	GetSeasonByPath(ctx context.Context, path string) (models.Season, error)
	SaveSeason(ctx context.Context, season *models.Season) (int64, error)
	GetEpisodesBySeasonID(ctx context.Context, seasonID int64) ([]models.Episode, error)
	GetEpisodeByPath(ctx context.Context, path string) (models.Episode, error)
	SaveEpisode(ctx context.Context, episode *models.Episode) (int64, error)
}
