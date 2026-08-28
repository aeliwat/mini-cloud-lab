# Build stage
FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /minicloud .

# Runtime stage — tiny image, CLI as entrypoint
FROM alpine:3.21

COPY --from=build /minicloud /usr/local/bin/minicloud

ENTRYPOINT ["minicloud"]
