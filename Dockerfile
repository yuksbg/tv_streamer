
## Build golang app
FROM golang:1.25 AS builder
ARG VersionTag=dev
COPY . /go/src/tv_streamer
WORKDIR /go/src/tv_streamer

RUN go mod tidy
# Build with version embedded from ARG
RUN CGO_ENABLED=0 go build -v -a -installsuffix cgo -o tv_streamer .


FROM alpine:3.22
RUN apk --no-cache add ca-certificates tzdata

RUN mkdir -p /app
COPY --from=builder /go/src/tv_streamer/tv_streamer /app/tv_streamer

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
# Set permissions and ownership
RUN chown appuser:appgroup /app

RUN chmod 755 /app/tv_streamer

USER appuser
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/app/tv_streamer"]