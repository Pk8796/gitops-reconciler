FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/web .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/web /web
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/web"]
