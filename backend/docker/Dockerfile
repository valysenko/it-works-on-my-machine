FROM golang:1.24.3-alpine3.20 AS builder

WORKDIR /src

COPY main.go monitoring.go fibonacci.go memory.go ping.go root.go video.go ./
COPY go.mod go.sum ./
RUN go mod download
RUN go mod verify
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/runner ./...

FROM scratch
COPY --from=builder src/bin/runner bin/runner
CMD ["bin/runner"]

