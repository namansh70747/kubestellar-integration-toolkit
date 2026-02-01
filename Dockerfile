# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY api/ api/
COPY pkg/ pkg/
COPY internal/ internal/

# Build the manager binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o ksit ./cmd/ksit/main.go

# Runtime stage - use distroless for security
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/ksit .
USER 65532:65532

ENTRYPOINT ["/ksit"]