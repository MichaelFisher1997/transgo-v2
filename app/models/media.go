package models

import (
	"database/sql"
)

// Media represents a media file
type Media struct {
	ID            int64          `db:"id"`
	Title         string         `db:"title"`
	Path          string         `db:"path"`
	MediaType     string         `db:"media_type"`
	FileSize      int64          `db:"file_size"`
	FileExtension string         `db:"file_extension"`
	PosterPath    sql.NullString `db:"poster_path"`
	Rating        sql.NullString `db:"rating"`
	Year          sql.NullInt64  `db:"year"`
	Description   sql.NullString `db:"description"`
}

// Media type constants
const (
	MediaTypeMovie  = "movie"
	MediaTypeTVShow = "tvshow"
)

// TVShow represents a TV show
type TVShow struct {
	ID          int64          `db:"id"`
	Title       string         `db:"title"`
	Path        string         `db:"path"`
	PosterPath  sql.NullString `db:"poster_path"`
	Rating      sql.NullString `db:"rating"`
	Year        sql.NullInt64  `db:"year"`
	Description sql.NullString `db:"description"`
}

// Season represents a TV show season
type Season struct {
	ID       int64  `db:"id"`
	TVShowID int64  `db:"tvshow_id"`
	Number   int    `db:"number"`
	Title    string `db:"title"`
	Path     string `db:"path"`
}

// Episode represents a TV show episode
type Episode struct {
	ID       int64          `db:"id"`
	SeasonID int64          `db:"season_id"`
	Number   int            `db:"number"`
	Title    string         `db:"title"`
	Path     string         `db:"path"`
	FileSize int64          `db:"file_size"`
	Rating   sql.NullString `db:"rating"`
}
