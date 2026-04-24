FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /markov ./cmd/markov

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash coreutils curl git jq ca-certificates procps \
  && rm -rf /var/lib/apt/lists/*
COPY --from=build /markov /usr/local/bin/markov
ENTRYPOINT ["markov"]
