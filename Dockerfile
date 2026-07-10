# ---- build stage ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache modules first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Version metadata (override with --build-arg).
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

# ---- runtime stage (distroless-style scratch, static, non-root) ----
FROM scratch
# CA certs (for git-over-HTTPS backup pushes) and a passwd with a non-root user.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/tasksd /usr/local/bin/tasksd
COPY --from=build /out/tasks  /usr/local/bin/tasks

# Run as an unprivileged uid; /data is the writable volume for the SQLite db.
USER 65532:65532
WORKDIR /data
VOLUME ["/data"]

ENV TASKS_ADDR=0.0.0.0:7842 \
    TASKS_DB=/data/tasks.db \
    TASKS_LOG_FORMAT=json
EXPOSE 7842

# Liveness: the static tasksd binary can't curl itself; use an external probe or
# the compose healthcheck below. Entry point serves by default.
ENTRYPOINT ["/usr/local/bin/tasksd"]
