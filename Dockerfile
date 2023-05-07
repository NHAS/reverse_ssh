FROM golang:1.20.4-bullseye

ARG WINDOWS_SUPPORT=true

WORKDIR /app

ENV DEBIAN_FRONTEND=noninteractive
ENV PATH="${PATH}:$(go env GOPATH)/bin"

RUN apt-get update && \
    if $WINDOWS_SUPPORT; then \
        apt-get install -y gcc-mingw-w64; \
    fi && \
    apt-get install -y upx-ucl && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN go install mvdan.cc/garble@89facf1

COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .

RUN make server

RUN chmod +x /app/docker-entrypoint.sh

ENTRYPOINT ["/app/docker-entrypoint.sh"]
