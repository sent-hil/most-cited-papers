FROM golang:1.24.1-alpine

# Install build dependencies for CGO
RUN apk add --no-cache gcc musl-dev sqlite

# Set working directory
WORKDIR /app

# Copy server files
COPY . /app/

# Enable CGO for SQLite support
ENV CGO_ENABLED=1

# Expose the default port
EXPOSE 9001

# Command to run the application
# Note: The database will be mounted when running the container
CMD ["go", "run", "ui_server.go", "template.go", "main.go", "-db=/data/paper_cache.db"]