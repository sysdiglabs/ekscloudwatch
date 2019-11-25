FROM golang:1.13-buster

WORKDIR /app

COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o ekscloudwatch github.com/sysdiglabs/ekscloudwatch/cmd

CMD ["./ekscloudwatch"]