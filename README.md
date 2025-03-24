# go-hn

A modern Hacker News client written in Go, providing a clean and efficient interface to browse Hacker News stories, comments, and user profiles.

## Features

- Browse top stories with pagination
- View individual stories and their comments
- User profile pages
- Login functionality
- Story submission capability
- Modern, responsive UI with HTMX integration
- Static file embedding for easy deployment
- Clean and efficient Go implementation

## Prerequisites

- Go 1.23.2 or later
- Make (optional, for using Makefile commands)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/tluyben/go-hn.git
cd go-hn
```

2. Build the project:
```bash
make build
```

Or without Make:
```bash
go build -o go-hn main.go
```

## Usage

Run the server:
```bash
make run
```

Or without Make:
```bash
go run main.go
```

The server will start on `http://localhost:8080`

## Development

- `make build` - Build the binary
- `make run` - Run the application
- `make clean` - Clean build artifacts
- `make test` - Run tests

## Project Structure

- `main.go` - Main application entry point
- `templates/` - HTML templates
- `static/` - Static assets (CSS, JavaScript)
- `hn/` - Hacker News API client implementation

## License

MIT License - see LICENSE file for details

## Author

- GitHub: [tluyben](https://github.com/tluyben)
