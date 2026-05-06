package main

import (
	"context"
	"transgo-v2/backend/models"
)

// MediaRepository defines the methods for database operations
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

	// Encode job methods
	CreateEncodeJob(ctx context.Context, job *models.EncodeJob) (int64, error)
	GetEncodeJob(ctx context.Context, id int64) (models.EncodeJob, error)
	GetEncodeJobs(ctx context.Context, limit, offset int) ([]models.EncodeJob, error)
	GetQueuedEncodeJobs(ctx context.Context) ([]models.EncodeJob, error)
	HasActiveJobForSource(ctx context.Context, sourcePath string) (bool, error)
	UpdateEncodeJobStatus(ctx context.Context, id int64, status string, completedFiles int, errorMsg *string) error
	CompleteEncodeJob(ctx context.Context, id int64) error
	CancelEncodeJob(ctx context.Context, id int64) error
	MarkEncodeJobCancelling(ctx context.Context, id int64) error
	UpdateJobProgress(ctx context.Context, id int64, currentFile string, progress float64, fps float64, speed string, processedBytes int64, processedFrames int64, totalFrames int64, currentFilePercent float64, etaSeconds int) error
	AddJobEvent(ctx context.Context, jobID int64, level, message string, filePath *string) error
	GetJobEvents(ctx context.Context, jobID int64, limit int) ([]models.EncodeJobEvent, error)
	DeleteEncodeJob(ctx context.Context, id int64) error

	// Encode result methods
	CreateEncodeResult(ctx context.Context, result *models.EncodeResult) (int64, error)
	GetEncodeResultByJobID(ctx context.Context, jobID int64) (models.EncodeResult, error)
	GetEncodeResults(ctx context.Context, limit, offset int) ([]models.EncodeResult, error)

	// Encode file result methods
	CreateEncodeFileResult(ctx context.Context, result *models.EncodeFileResult) (int64, error)
	GetEncodeFileResultsByResultID(ctx context.Context, resultID int64) ([]models.EncodeFileResult, error)
	UpdateEncodeFileResult(ctx context.Context, id int64, encodedSize int64, status string) error
	StartEncodeFileResult(ctx context.Context, id int64) error
	FinishEncodeFileResult(ctx context.Context, id int64, encodedSize, spaceSaved int64, verificationStatus string) error
	FailEncodeFileResult(ctx context.Context, id int64, message, logTail string) error
}
