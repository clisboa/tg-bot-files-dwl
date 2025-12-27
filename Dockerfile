FROM golang:1.25.5 AS builder
WORKDIR /build
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /build/tg-bot-files-dwl .

###########################################################
# The *final* image

FROM gcr.io/distroless/static
COPY --from=builder /build/tg-bot-files-dwl /tg-bot-files-dwl
CMD ["/tg-bot-files-dwl"]
