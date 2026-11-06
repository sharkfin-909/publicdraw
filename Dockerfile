FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /publicdraw ./cmd/publicdraw

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 publicdraw && mkdir /data && chown publicdraw /data
COPY --from=build /publicdraw /usr/local/bin/publicdraw
USER publicdraw
WORKDIR /data
VOLUME /data
ENTRYPOINT ["publicdraw", "-dir", "/data"]
CMD ["serve", "-addr", ":8080"]
