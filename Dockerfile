FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/watch-tower ./cmd/watch-tower

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/watch-tower /usr/local/bin/watch-tower
ENTRYPOINT ["/usr/local/bin/watch-tower"]

