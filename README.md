# TransGo Media Manager (Early Alpha)

🚧 **Warning: Early Development Stage** 🚧  
This project is currently in active development. Expect:
- Breaking changes
- Missing features
- Potential instability
- Planned features may change significantly

We welcome contributors and early testers, but please don't use in production yet.  
All feature plans are tentative and subject to change as development progresses.

A modern media management system with powerful encoding capabilities.

## Key Features

### Media Management
- Supports modern formats including AV1, HEVC, VP9  
- Automatic media library organization
- Metadata extraction and tagging
- Cross-platform compatibility

### Advanced Encoding
- Mass concurrent encoding pipeline  
- Hardware acceleration support (NVENC, QuickSync)  
- Adaptive bitrate streaming outputs  
- Batch processing workflows  

### Technical Highlights
- Go backend for high performance  
- Templ/Tailwind frontend  
- PostgreSQL database  
- CI/CD with GitHub Actions  
- Containerized deployment  

## Getting Started

### Prerequisites
- Go 1.24+  
- PostgreSQL 15+  
- FFmpeg with AV1 support  

### Installation
```bash
git clone https://github.com/your-repo/transgo-v2.git
cd transgo-v2
go build ./...
```

### Configuration
Copy `.env.example` to `.env` and configure:
```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
ENCODER_THREADS=4  # Set based on CPU cores
```

## Usage
Start the server:
```bash
./transgo
```

Access the web interface at `http://localhost:8080`

## Development

Use the hot-reload dev script while building the app:

```bash
./dev.sh
```

- Frontend with Vite HMR: `http://localhost:5173`
- Backend API with source restart: `http://localhost:8080`
- API calls from Vite are proxied to the backend.
- Go source changes restart the backend automatically with `go run .`.
- React/TypeScript changes reload in the browser automatically.
- Docker is only used for Postgres in development.
- No manual frontend build, Go binary copy, or production image rebuild is needed during development.

Run tests:
```bash
cd backend && go test ./... -v
```

Production build:
```bash
./build.sh
docker compose up --build
```

## License
MIT
