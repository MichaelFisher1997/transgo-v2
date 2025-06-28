package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/MichaelFisher1997/transgo-v2/app/models"
)

// MediaFile represents a media file
type MediaFile struct {
	Path string
	Size int64
}

// ScanMediaDirectory scans a directory for media files
func ScanMediaDirectory(dir, mediaType string) ([]MediaFile, error) {
	var files []MediaFile
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error walking path %s: %v", path, err)
			return err
		}
		if !info.IsDir() {
			// Check if it's a video file
			ext := strings.ToLower(filepath.Ext(path))
			if isVideoFile(ext) {
				files = append(files, MediaFile{Path: path, Size: info.Size()})
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error walking directory %s: %v", dir, err)
		return nil, err
	}
	return files, nil
}

// isVideoFile checks if a file extension is a video format
func isVideoFile(ext string) bool {
	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v"}
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	return false
}

// ScanMedia scans all media directories
func ScanMedia(repo MediaRepository, moviesDir, tvDir string) {

	// Scan movies
	ScanMovies(repo, moviesDir)

	// Scan TV shows
	ScanTVShows(repo, tvDir)

	fmt.Println("Media scan complete")
}

// ScanMovies scans the movies directory
func ScanMovies(repo MediaRepository, moviesDir string) {
	movies, err := ScanMediaDirectory(moviesDir, models.MediaTypeMovie)
	if err != nil {
		log.Printf("Scan failed for %s: %v", moviesDir, err)
		return
	}

	for _, movie := range movies {
		_, err := repo.GetMediaByPath(context.Background(), movie.Path)
		if err == nil {
			continue // File already exists
		}

		media := &models.Media{
			Title:         cleanTitle(filepath.Base(movie.Path)),
			Path:          movie.Path,
			MediaType:     models.MediaTypeMovie,
			FileSize:      movie.Size,
			FileExtension: filepath.Ext(movie.Path),
		}
		if _, err := repo.SaveMedia(context.Background(), media); err != nil {
			log.Printf("Error saving media: %v", err)
		}
	}
}

// ScanTVShows scans the TV shows directory
func ScanTVShows(repo MediaRepository, tvDir string) {

	// Get all TV show directories
	tvShows, err := os.ReadDir(tvDir)
	if err != nil {
		log.Printf("Error reading TV directory: %v", err)
		return
	}

	for _, tvShowDir := range tvShows {
		if !tvShowDir.IsDir() {
			continue
		}

		tvShowPath := filepath.Join(tvDir, tvShowDir.Name())
		tvShowTitle := tvShowDir.Name()

		// Check if TV show already exists
		tvShow, err := repo.GetTVShowByPath(context.Background(), tvShowPath)
		if err != nil {
			// Create new TV show
			newTVShow := &models.TVShow{
				Title: tvShowTitle,
				Path:  tvShowPath,
			}
			tvShowID, err := repo.SaveTVShow(context.Background(), newTVShow)
			if err != nil {
				log.Printf("Error saving TV show: %v", err)
				continue
			}
			tvShow.ID = tvShowID
		}

		// Scan for seasons
		scanSeasons(repo, tvShow.ID, tvShowPath)
	}
}

// scanSeasons scans for seasons within a TV show directory
func scanSeasons(repo MediaRepository, tvShowID int64, tvShowPath string) {
	// Check for season directories
	entries, err := os.ReadDir(tvShowPath)
	if err != nil {
		log.Printf("Error reading TV show directory: %v", err)
		return
	}

	// Regular expression to match "Season X" or "SX" directories
	seasonRegex := regexp.MustCompile(`(?i)(season|s)\s*(\d+)`)

	// First, look for season directories
	hasSeasonDirs := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		matches := seasonRegex.FindStringSubmatch(dirName)
		if len(matches) > 2 {
			hasSeasonDirs = true
			seasonNum := 0
			fmt.Sscanf(matches[2], "%d", &seasonNum)
			seasonPath := filepath.Join(tvShowPath, dirName)

			// Check if season already exists
			season, err := repo.GetSeasonByPath(context.Background(), seasonPath)
			if err != nil {
				// Create new season
				newSeason := &models.Season{
					TVShowID: tvShowID,
					Number:   seasonNum,
					Title:    fmt.Sprintf("Season %d", seasonNum),
					Path:     seasonPath,
				}
				seasonID, err := repo.SaveSeason(context.Background(), newSeason)
				if err != nil {
					log.Printf("Error saving season: %v", err)
					continue
				}
				season.ID = seasonID
			}

			// Scan for episodes in this season
			scanEpisodes(repo, season.ID, seasonPath)
		}
	}

	// If no season directories found, treat the TV show directory as a single season
	if !hasSeasonDirs {
		// Check if default season already exists
		seasonPath := tvShowPath
		season, err := repo.GetSeasonByPath(context.Background(), seasonPath)
		if err != nil {
			// Create default season
			newSeason := &models.Season{
				TVShowID: tvShowID,
				Number:   1,
				Title:    "Season 1",
				Path:     seasonPath,
			}
			seasonID, err := repo.SaveSeason(context.Background(), newSeason)
			if err != nil {
				log.Printf("Error saving default season: %v", err)
				return
			}
			season.ID = seasonID
		}

		// Scan for episodes in the TV show directory
		scanEpisodes(repo, season.ID, seasonPath)
	}
}

// scanEpisodes scans for episodes within a season directory
func scanEpisodes(repo MediaRepository, seasonID int64, seasonPath string) {
	// Get all files in the season directory
	files, err := ScanMediaDirectory(seasonPath, models.MediaTypeTVShow)
	if err != nil {
		log.Printf("Error scanning season directory: %v", err)
		return
	}

	for _, file := range files {
		// Check if episode already exists
		_, err := repo.GetEpisodeByPath(context.Background(), file.Path)
		if err == nil {
			continue // Episode already exists
		}

		// Extract episode information
		_, episodeNum, title := ExtractEpisodeInfo(file.Path)

		// Create new episode
		newEpisode := &models.Episode{
			SeasonID: seasonID,
			Number:   episodeNum,
			Title:    title,
			Path:     file.Path,
			FileSize: file.Size,
		}

		if _, err := repo.SaveEpisode(context.Background(), newEpisode); err != nil {
			log.Printf("Error saving episode: %v", err)
		}
	}
}

// cleanTitle removes file extensions and common suffixes from a title
func cleanTitle(filename string) string {
	// Handle hidden files like ".bashrc" or ".gitignore"
	if strings.HasPrefix(filename, ".") && !strings.Contains(filename[1:], ".") {
		return filename
	}

	// Remove file extension
	title := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Define a more comprehensive set of suffix patterns to remove
	// Order matters: more specific patterns first
	suffixPatterns := []string{
		`(?i)\s*\[\s*\d{3,4}p\s*\]`, // [1080p], [720p]
		`(?i)\s*\(\s*\d{3,4}p\s*\)`, // (1080p), (720p)
		`(?i)\s*-\s*\d{3,4}p`,       // - 1080p
		`(?i)\s*\d{3,4}p`,           // 1080p (without leading dash/space)
		`(?i)\s*\[\s*HD\s*\]`,       // [HD]
		`(?i)\s*\(\s*HD\s*\)`,       // (HD)
		`(?i)\s*BluRay`,             // BluRay
		`(?i)\s*WEB-DL`,             // WEB-DL
		`(?i)\s*WEBRip`,             // WEBRip
		`(?i)\s*HDRip`,              // HDRip
		`(?i)\s*BDRip`,              // BDRip
		`(?i)\s*DVDRip`,             // DVDRip
		`(?i)\s*x264`,               // x264
		`(?i)\s*x265`,               // x265
		`(?i)\s*HEVC`,               // HEVC
		`(?i)\s*AAC`,                // AAC
		`(?i)\s*AC3`,                // AC3
		`(?i)\s*DTS`,                // DTS
		`(?i)\s*YIFY`,               // YIFY
		`(?i)\s*RARBG`,              // RARBG
		`(?i)\s*-RARBG`,             // -RARBG
		`(?i)\s*\[[^\]]+\]`,         // [Anything in brackets]
		`(?i)\s*\([^\)]+\)`,         // (Anything in parentheses)
		`(?i)\s*-\s*[^-\s]+$`,       // - GroupName at the end
		`(?i)\s*\.\s*[^.\s]+$`,      // .GroupName at the end
		`(?i)\s*\d{4}`,              // Year like 2023 (should be after resolution/quality)
	}

	for _, pattern := range suffixPatterns {
		re := regexp.MustCompile(pattern)
		title = re.ReplaceAllString(title, "")
	}

	// Replace dots and underscores with spaces
	title = strings.ReplaceAll(title, ".", " ")
	title = strings.ReplaceAll(title, "_", " ")

	// Trim spaces
	title = strings.TrimSpace(title)

	// Remove any trailing dashes and spaces
	title = strings.TrimRight(title, " -")

	return title
}

// ExtractEpisodeInfo extracts episode number and title from a file path
func ExtractEpisodeInfo(filePath string) (seasonNum, episodeNum int, title string) {
	// Example: "TV.Show.S01E05.Episode.Title.mp4"
	// Example: "TV.Show.Season.1.Episode.5.Episode.Title.mp4"
	// Example: "TV.Show.E05.Episode.Title.mp4" (assuming single season)

	filename := filepath.Base(filePath)
	cleanedTitle := cleanTitle(filename) // Use the existing cleanTitle function

	// Regex to find SXXEXX or Season X Episode X patterns
	re := regexp.MustCompile(`(?i)(s(\d+))?e(\d+)\.?(.+)`)
	matches := re.FindStringSubmatch(cleanedTitle)

	if len(matches) > 4 {
		// Found SXXEXX or EXX pattern
		if matches[2] != "" {
			seasonNum, _ = strconv.Atoi(matches[2])
		} else {
			seasonNum = 1 // Default to season 1 if not specified
		}
		episodeNum, _ = strconv.Atoi(matches[3])
		title = strings.TrimSpace(strings.ReplaceAll(matches[4], ".", " "))
	} else {
		// Fallback if no clear SXXEXX or EXX pattern, try to extract just episode number
		re = regexp.MustCompile(`(?i)episode\.?(\d+)\.?(.+)`)
		matches = re.FindStringSubmatch(cleanedTitle)
		if len(matches) > 2 {
			episodeNum, _ = strconv.Atoi(matches[1])
			title = strings.TrimSpace(strings.ReplaceAll(matches[2], ".", " "))
		} else {
			// No clear episode info, use cleaned filename as title
			title = cleanedTitle
			episodeNum = 0 // Indicate no episode number found
		}
	}

	return seasonNum, episodeNum, title
}
