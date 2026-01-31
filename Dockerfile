FROM golang:1.21 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o ksit ./cmd/ksit/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates && \
    addgroup -g 65532 -S nonroot && \
    adduser -u 65532 -S nonroot -G nonroot
WORKDIR /
COPY --from=builder /app/ksit /ksit
USER 65532:65532
ENTRYPOINT ["/ksit"]