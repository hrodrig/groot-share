# syntax=docker/dockerfile:1
# Local / CI image: compile inside Docker (make docker-build).
# Release images: GoReleaser builds static binaries, then Dockerfile.release packages them.
FROM --platform=$BUILDPLATFORM golang:1.26.5-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG APP_VERSION=0.0.0
ARG GIT_COMMIT=unknown
ARG GIT_BRANCH=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
	-ldflags="-s -w -X main.version=${APP_VERSION} -X main.commit=${GIT_COMMIT} -X main.branch=${GIT_BRANCH} -X main.buildDate=${BUILD_DATE}" \
	-o /out/gfs ./cmd/gfs

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=builder /out/gfs /app/gfs
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/gfs"]
