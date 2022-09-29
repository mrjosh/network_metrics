FROM golang:1.18.1-alpine AS builder

LABEL maintainer="Alireza Josheghani <josheghani.dev@gmail.com>"

# Creating work directory
WORKDIR /build

# Adding project to work directory
ADD . /build

ENV PUID 1000
ENV PGID 1000

# build project
RUN go build -o server .

FROM alpine AS app

WORKDIR /
COPY --from=builder /build/server /server

EXPOSE 9091

ENTRYPOINT ["/server"]
