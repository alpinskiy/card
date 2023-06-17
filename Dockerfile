FROM golang:1.19 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /kdiscovery

FROM gcr.io/distroless/base-debian11 AS release
WORKDIR /
COPY --from=build /kdiscovery /kdiscovery
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/kdiscovery"]
