# Render deploy image. Unlike the minimal scratch image, this is alpine-based so
# it has a shell + git for the git-backup durability layer (Render free has no
# persistent disk, so the DB is reconstructed from a private backup repo on boot
# and pushed back on every change).
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG COMMIT=""
ARG DATE=""
ENV CGO_ENABLED=0
RUN go build -trimpath \
      -ldflags "-s -w \
        -X github.com/tristanMatthias/tasks/internal/buildinfo.Version=${VERSION} \
        -X github.com/tristanMatthias/tasks/internal/buildinfo.Commit=${COMMIT} \
        -X github.com/tristanMatthias/tasks/internal/buildinfo.Date=${DATE}" \
      -o /out/tasksd ./cmd/tasksd \
 && go build -trimpath -ldflags "-s -w" -o /out/tasks ./cmd/tasks

FROM alpine:3.20
RUN apk add --no-cache git openssh-client ca-certificates tini && adduser -D -u 65532 app
COPY --from=build /out/tasksd /usr/local/bin/tasksd
COPY --from=build /out/tasks  /usr/local/bin/tasks
COPY deploy/render/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh && mkdir -p /data && chown app /data
USER app
ENV TASKS_DB=/data/tasks.db TASKS_LOG_FORMAT=json
# tini reaps zombies and forwards SIGTERM so the exporter flushes + git-pushes on shutdown.
ENTRYPOINT ["/sbin/tini","--","/usr/local/bin/entrypoint.sh"]
