FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The private admin build (admin.go, git-ignored) is enabled only when the file
# is present; a clean clone builds the public binary with the no-op stub.
RUN TAGS=""; [ -f admin.go ] && TAGS="admin"; \
    CGO_ENABLED=0 go build -tags "$TAGS" -ldflags="-s -w" -o monitor .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/monitor .

VOLUME /app/data
EXPOSE 8080
CMD ["./monitor"]
