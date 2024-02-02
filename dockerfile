# Use the official Golang image as the base image
FROM golang:latest

# Set the working directory inside the container
WORKDIR /app

# Copy the local files to the container's working directory
COPY . .

# Build the Go application
RUN go build -o gdrivesync

# Entry point command to run the application with environment variables
CMD ["./gdrivesync"]

# Example usage:
# docker build -t gdrivesync .
# docker run -e SYNC_PATH=/default/sync/path -e API_KEY=your_api_key -v /local/sync/path:/default/sync/path sync-app
