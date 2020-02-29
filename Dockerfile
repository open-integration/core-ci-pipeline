FROM golang:1.13.5-alpine3.10 AS godev

RUN apk update && apk add --no-cache ca-certificates && apk upgrade && apk add git make

WORKDIR /core-ci

COPY . .

# https://github.com/kubernetes/test-infra/issues/16116
RUN go get github.com/googleapis/gnostic@v0.4.0

RUN make build

FROM alpine:3.9

COPY VERSION .

RUN apk update && apk add --no-cache ca-certificates && apk upgrade

COPY --from=godev ./core-ci/core-ci /core-ci