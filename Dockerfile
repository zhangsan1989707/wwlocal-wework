FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/config.yaml .
RUN mkdir -p keys && chown nobody:nobody keys

EXPOSE 8080

USER nobody

CMD ["./server"]