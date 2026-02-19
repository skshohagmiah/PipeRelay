FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /bin/piperelay ./cmd/piperelay

FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs
COPY --from=builder /bin/piperelay /usr/local/bin/piperelay

RUN mkdir -p /data
VOLUME /data

EXPOSE 8080

ENTRYPOINT ["piperelay"]
CMD ["serve"]
