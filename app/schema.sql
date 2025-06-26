CREATE TABLE media (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    path TEXT NOT NULL,
    media_type TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    file_extension TEXT NOT NULL,
    poster_path TEXT,
    rating TEXT,
    year INTEGER,
    description TEXT
);

CREATE TABLE tvshows (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    path TEXT NOT NULL,
    poster_path TEXT,
    rating TEXT,
    year INTEGER,
    description TEXT
);

CREATE TABLE seasons (
    id SERIAL PRIMARY KEY,
    tvshow_id INTEGER NOT NULL REFERENCES tvshows(id) ON DELETE CASCADE,
    number INTEGER NOT NULL,
    title TEXT NOT NULL,
    path TEXT NOT NULL
);

CREATE TABLE episodes (
    id SERIAL PRIMARY KEY,
    season_id INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    number INTEGER NOT NULL,
    title TEXT NOT NULL,
    path TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    rating TEXT
);
