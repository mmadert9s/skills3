FROM golang:1.22 as build

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 go build -v -o /usr/local/bin/web ./...

FROM alpine:latest

COPY --from=build /usr/local/bin/web /usr/local/bin/web
COPY "index.html" "index.html"

CMD ["web"]