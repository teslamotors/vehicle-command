FROM golang:1.20 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir build
RUN go build -o ./build ./...

FROM gcr.io/distroless/base-debian12 AS runtime

COPY --from=build /app/build /usr/local/bin

ENTRYPOINT ["tesla-http-proxy"]
