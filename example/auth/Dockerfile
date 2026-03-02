FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN go build -o /app .

FROM alpine:3.21
COPY --from=build /app /app
EXPOSE 50051
CMD ["/app"]
