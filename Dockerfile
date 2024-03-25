FROM golang:bullseye

WORKDIR /app

RUN apt update -y
RUN apt upgrade -y
RUN apt install -y upx-ucl gcc-mingw-w64

RUN go install mvdan.cc/garble@latest

ENV PATH="${PATH}:$(go env GOPATH)/bin"

COPY go.mod go.sum ./
RUN go mod download -x

COPY . .
RUN make server

EXPOSE 2222

RUN chmod +x /app/docker-entrypoint.sh

ENTRYPOINT ["/app/docker-entrypoint.sh"]