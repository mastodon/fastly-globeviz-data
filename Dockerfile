# syntax=docker/dockerfile:1

# Build the application from source
FROM golang:1.21 AS build-stage

WORKDIR /app

# COPY go.mod go.sum ./
# RUN go mod download

COPY go.mod *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /fastly-globeviz-data

# Deploy the application binary into a lean image
FROM gcr.io/distroless/base-debian12 AS build-release-stage

WORKDIR /

COPY --from=build-stage /fastly-globeviz-data /fastly-globeviz-data

EXPOSE 4000

USER nonroot:nonroot

ENTRYPOINT ["/fastly-globeviz-data"]
