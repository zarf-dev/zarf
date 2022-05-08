FROM golang:1.18 as build
WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY src src

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X 'github.com/defenseunicorns/zarf/src/config.CLIVersion=container'" -o /zarf src/main.go

FROM gcr.io/distroless/base
COPY --from=build /zarf /
EXPOSE 8443

CMD ["/zarf", "agent", "-l=trace"]