FROM golang:latest as builder

WORKDIR /build
COPY server.go /build/
RUN go version
ARG VERSION=0.0.0-development
RUN cd /build && \
    go get -v -t -d ./... && \
    CGO_ENABLED=0 go build -ldflags "-X main.version=$VERSION" -v -o server .

FROM alpine:3.9.4
RUN adduser -S -D -H server
USER server
COPY --from=builder /build /server
COPY schema.json /server
EXPOSE 3051/tcp
EXPOSE 3050/udp
EXPOSE 3060/tcp
WORKDIR /server
CMD ["./server"]
ARG AWS_BUCKET
ARG AWS_ACCESS_KEY_ID
ARG AWS_SECRET_ACCESS_KEY
ENV AWS_BUCKET=$AWS_BUCKET
ENV AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID
ENV AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY
