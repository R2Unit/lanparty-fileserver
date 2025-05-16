FROM golang:1.24.3-alpine AS builder

WORKDIR /app

COPY go.mod ./

RUN go mod download && go mod verify

COPY cmd/ ./cmd/

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-s -w" -o /lanparty-fileserver ./cmd/lanparty-fileserver/

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

RUN groupadd -r robot --gid 1001 && \
    useradd --no-log-init -r -m -d /app -s /sbin/nologin -g robot --uid 1001 robot

WORKDIR /app

COPY templates/ ./templates/
COPY preloaded-games/ ./preloaded-games/

COPY --from=builder /lanparty-fileserver /usr/local/bin/lanparty-fileserver

RUN chown -R robot:robot /app && \
    chmod -R u+rwx,g+rsx /app # User has rwx, group has rx and setgid for new files/dirs in group

USER robot

EXPOSE 80

CMD ["/usr/local/bin/lanparty-fileserver"]