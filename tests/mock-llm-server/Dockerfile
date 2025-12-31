FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY main.go .
RUN go build -o mock-llm-server main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/mock-llm-server .

ENV PORT=8090
EXPOSE 8090

CMD ["./mock-llm-server"]
