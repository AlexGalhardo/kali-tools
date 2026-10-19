FROM golang:1.27.2-bookworm AS source
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM source AS test
ENV CGO_ENABLED=1 GOCACHE=/tmp/go-cache GOTMPDIR=/tmp
RUN mkdir -p /tmp/go-cache && chmod 1777 /tmp/go-cache
USER 65532:65532
CMD ["go", "test", "-race", "-tags", "integration", "-timeout", "90s", "./internal/...", "./tests/integration/..."]

FROM source AS build
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/hacker-tools ./cmd/hacker-tools && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/lab-fixture ./cmd/lab-fixture

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/hacker-tools /usr/local/bin/hacker-tools
COPY --from=build /out/lab-fixture /usr/local/bin/lab-fixture
USER 65532:65532
ENTRYPOINT ["hacker-tools"]
CMD ["doctor"]
