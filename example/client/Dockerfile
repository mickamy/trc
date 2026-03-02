FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN go build -o /app .

FROM alpine:3.21
COPY --from=build /app /app
CMD ["/app"]
