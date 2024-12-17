# Stage 1
FROM golang:1.23 as builder
WORKDIR /go/src/app
COPY . .
RUN go mod tidy
RUN go mod vendor
RUN CGO_ENABLED=0 go build -o /go/bin/app/credhub_exporter -a -tags netgo -ldflags '-w -extldflags "-static"'

# Stage 2
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /go/bin/app/credhub_exporter /bin/credhub_exporter
EXPOSE 9358
ENTRYPOINT ["/bin/credhub_exporter"]
