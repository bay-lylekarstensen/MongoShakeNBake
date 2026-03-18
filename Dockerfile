FROM golang:1.24-alpine3.20 as builder
RUN apk add --no-cache bash make git zip tzdata ca-certificates gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make linux

FROM alpine:3.20
# Dependencies
RUN apk --no-cache add tzdata ca-certificates musl
# where application lives
WORKDIR /app
# Copy the products
COPY --from=builder /app/bin/collector /app/collector
COPY --from=builder /app/bin/receiver /app/receiver
COPY --from=builder /app/bin/start.sh /app/start.sh
COPY --from=builder /app/bin/stop.sh /app/stop.sh
COPY --from=builder /app/bin/mongoshake-stat /app/mongoshake-stat
COPY --from=builder /app/conf /app/conf
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh
ENV MSB_CONF=/app/conf/collector.conf
# metrics
EXPOSE 9100
ENTRYPOINT ["/app/docker-entrypoint.sh"]