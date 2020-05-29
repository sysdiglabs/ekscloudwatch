FROM golang:1.13-buster as build

RUN apt-get update && apt-get install apt ca-certificates -y && \
    useradd -u 1000 ekscloudwatch

WORKDIR /app

COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/ekscloudwatch github.com/sysdiglabs/ekscloudwatch/cmd && \
    chmod +x /app/ekscloudwatch

FROM scratch

COPY --from=build /app/ekscloudwatch /ekscloudwatch
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /etc/group /etc/group

USER 1000

CMD ["/ekscloudwatch"]
