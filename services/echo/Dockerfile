FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /out/echo .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/echo /echo
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/echo"]
