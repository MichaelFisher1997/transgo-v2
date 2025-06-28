package main

import (
	"context"
	"fmt"
	"os"

	"github.com/MichaelFisher1997/transgo-v2/app/models"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// DBTX represents a database transaction
type DBTX interface {
	sqlx.ExtContext
	sqlx.PreparerContext
}

// Repository holds the database connection
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates a new Repository
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Config holds the database configuration
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// NewConfig creates a new Config
func NewConfig() *Config {
	return &Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
	}
}

// NewDB creates a new database connection
func NewDB(cfg *Config) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// GetAllMedia retrieves all media from the database
func (r *Repository) GetAllMedia(ctx context.Context) ([]models.Media, error) {
	var media []models.Media
	err := r.db.SelectContext(ctx, &media, "SELECT * FROM media")
	if err != nil {
		return nil, err
	}
	return media, nil
}

// GetMediaByType retrieves all media of a specific type from the database
func (r *Repository) GetMediaByType(ctx context.Context, mediaType string) ([]models.Media, error) {
	var media []models.Media
	err := r.db.SelectContext(ctx, &media, "SELECT * FROM media WHERE media_type = $1", mediaType)
	if err != nil {
		return nil, err
	}
	return media, nil
}

// GetMediaByPath retrieves a media file by its path
func (r *Repository) GetMediaByPath(ctx context.Context, path string) (models.Media, error) {
	var media models.Media
	err := r.db.GetContext(ctx, &media, "SELECT * FROM media WHERE path = $1", path)
	return media, err
}

// SaveMedia saves a media file to the database
func (r *Repository) SaveMedia(ctx context.Context, media *models.Media) (int64, error) {
	query := `INSERT INTO media (title, path, media_type, file_size, file_extension)
	VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var id int64
	err := r.db.QueryRowxContext(ctx, query, media.Title, media.Path, media.MediaType, media.FileSize, media.FileExtension).Scan(&id)
	return id, err
}

// GetAllTVShows retrieves all TV shows from the database
func (r *Repository) GetAllTVShows(ctx context.Context) ([]models.TVShow, error) {
	var tvshows []models.TVShow
	err := r.db.SelectContext(ctx, &tvshows, "SELECT * FROM tvshows")
	if err != nil {
		return nil, err
	}
	return tvshows, nil
}

// GetTVShowByID retrieves a TV show by its ID
func (r *Repository) GetTVShowByID(ctx context.Context, id int64) (models.TVShow, error) {
	var tvshow models.TVShow
	err := r.db.GetContext(ctx, &tvshow, "SELECT * FROM tvshows WHERE id = $1", id)
	return tvshow, err
}

// GetTVShowByPath retrieves a TV show by its path
func (r *Repository) GetTVShowByPath(ctx context.Context, path string) (models.TVShow, error) {
	var tvshow models.TVShow
	err := r.db.GetContext(ctx, &tvshow, "SELECT * FROM tvshows WHERE path = $1", path)
	return tvshow, err
}

// SaveTVShow saves a TV show to the database
func (r *Repository) SaveTVShow(ctx context.Context, tvshow *models.TVShow) (int64, error) {
	query := `INSERT INTO tvshows (title, path)
	VALUES ($1, $2) RETURNING id`
	var id int64
	err := r.db.QueryRowxContext(ctx, query, tvshow.Title, tvshow.Path).Scan(&id)
	return id, err
}

// GetSeasonsByTVShowID retrieves all seasons for a TV show
func (r *Repository) GetSeasonsByTVShowID(ctx context.Context, tvshowID int64) ([]models.Season, error) {
	var seasons []models.Season
	err := r.db.SelectContext(ctx, &seasons, "SELECT * FROM seasons WHERE tvshow_id = $1 ORDER BY number", tvshowID)
	if err != nil {
		return nil, err
	}
	return seasons, nil
}

// GetSeasonByPath retrieves a season by its path
func (r *Repository) GetSeasonByPath(ctx context.Context, path string) (models.Season, error) {
	var season models.Season
	err := r.db.GetContext(ctx, &season, "SELECT * FROM seasons WHERE path = $1", path)
	return season, err
}

// SaveSeason saves a season to the database
func (r *Repository) SaveSeason(ctx context.Context, season *models.Season) (int64, error) {
	query := `INSERT INTO seasons (tvshow_id, number, title, path)
	VALUES ($1, $2, $3, $4) RETURNING id`
	var id int64
	err := r.db.QueryRowxContext(ctx, query, season.TVShowID, season.Number, season.Title, season.Path).Scan(&id)
	return id, err
}

// GetEpisodesBySeasonID retrieves all episodes for a season
func (r *Repository) GetEpisodesBySeasonID(ctx context.Context, seasonID int64) ([]models.Episode, error) {
	var episodes []models.Episode
	err := r.db.SelectContext(ctx, &episodes, "SELECT * FROM episodes WHERE season_id = $1 ORDER BY number", seasonID)
	if err != nil {
		return nil, err
	}
	return episodes, nil
}

// GetEpisodeByPath retrieves an episode by its path
func (r *Repository) GetEpisodeByPath(ctx context.Context, path string) (models.Episode, error) {
	var episode models.Episode
	err := r.db.GetContext(ctx, &episode, "SELECT * FROM episodes WHERE path = $1", path)
	return episode, err
}

// SaveEpisode saves an episode to the database
func (r *Repository) SaveEpisode(ctx context.Context, episode *models.Episode) (int64, error) {
	query := `INSERT INTO episodes (season_id, number, title, path, file_size)
	VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var id int64
	err := r.db.QueryRowxContext(ctx, query, episode.SeasonID, episode.Number, episode.Title, episode.Path, episode.FileSize).Scan(&id)
	return id, err
}
