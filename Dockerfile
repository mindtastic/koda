FROM golang:1.18-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

# Copy project files into container
COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
RUN go build -ldflags="-s -w" -o koda .

FROM scratch

COPY --from=builder ["/build/koda", "/"]

# Command to run when starting the container.
ENTRYPOINT ["/koda"]