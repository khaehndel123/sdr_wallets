###################################################
### sdr-wallet builder
###################################################

FROM	golang:1.12-alpine as build_base

RUN	    apk add --no-cache alpine-sdk git
ENV	    GO111MODULE=on
WORKDIR	/go/src/backend
COPY    go.mod .
COPY    go.sum .
RUN     go mod download

####################################################

FROM    build_base as builder_backend
COPY    . .
RUN     go build -o sdrwallet cmd/main.go

####################################################
###############  For run in docker  ################
####################################################
#FROM    alpine:latest as sdr-wallet
#COPY    --from=builder_backend /go/src/backend/sdrwallet /root/sdrwallet
#COPY    config.yaml /root/config.yaml
#CMD     [ "/root/sdrwallet", "--config", "/root/config.yaml" ]