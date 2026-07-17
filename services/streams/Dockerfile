FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/streams .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/streams /streams
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/streams"]
