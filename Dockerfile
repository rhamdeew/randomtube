FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -o randomtube .

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/randomtube .

EXPOSE 8080

ENTRYPOINT ["/app/randomtube"]
