FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /pg-monitor ./cmd/server

FROM alpine:3.20
COPY --from=build /pg-monitor /pg-monitor
EXPOSE 8080
CMD ["/pg-monitor"]
