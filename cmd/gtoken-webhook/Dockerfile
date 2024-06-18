# syntax = docker/dockerfile:experimental

#
# ----- Go Builder Image ------
#
FROM golang:1.22.2-alpine3.19 AS builder

ENV GOROOT /usr/local/go

# curl git bash
RUN apk add --no-cache curl git bash make
COPY --from=golangci/golangci-lint:v1.59.0-alpine /usr/bin/golangci-lint /usr/bin

#
# ----- Build and Test Image -----
#
FROM builder as build

# set working directorydoc
RUN mkdir -p /go/src/gtoken-webhook
WORKDIR /go/src/gtoken-webhook

# load dependency
COPY go.mod .
COPY go.sum .
RUN --mount=type=cache,target=/go/mod go mod download

# copy sources
COPY . .

# build
RUN make

#
# ------ get latest CA certificates
#
FROM alpine:3.19 as certs
RUN apk --update add ca-certificates


#
# ------ gtoken release Docker image ------
#
FROM scratch

# copy CA certificates
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# this is the last commabd since it's never cached
COPY --from=build /go/src/gtoken-webhook/.bin/gtoken-webhook /gtoken-webhook

ENTRYPOINT ["/gtoken-webhook"]