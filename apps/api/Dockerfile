FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.work ./
COPY apps/api ./apps/api
RUN go build -o /out/api ./apps/api/cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/api /app/api
EXPOSE 8080
ENTRYPOINT ["/app/api"]
