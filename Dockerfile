FROM golang:1.13 as builder

ENV CGO_ENABLED=0
ENV GOOS=linux 

ADD . /tmp/updater
WORKDIR /tmp/updater
RUN go build -o updater -ldflags '-s -w -extldflags "-static"' -mod=vendor .

# ---

FROM scratch
COPY --from=builder /tmp/updater/updater /updater
ENTRYPOINT ["/updater"]
