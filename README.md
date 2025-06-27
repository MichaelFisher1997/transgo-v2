# TransGo Media Manager

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
Run tests:
```bash
go test ./... -v
```

Build assets:
```bash
./build.sh
```

## License
MIT
