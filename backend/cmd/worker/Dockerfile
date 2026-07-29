FROM golang:1.26.5-alpine

WORKDIR /app

# Copy dependency files and download
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
RUN go build -o worker ./cmd/worker

CMD ["./worker"]
