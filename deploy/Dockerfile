# syntax=docker/dockerfile:1.7
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/vps-mcp ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/vps-mcp /usr/local/bin/vps-mcp
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/vps-mcp"]
