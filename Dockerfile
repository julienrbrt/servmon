FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o servmon .

FROM alpine:latest
COPY --from=builder /build/servmon /servmon
ENTRYPOINT ["/servmon"]
