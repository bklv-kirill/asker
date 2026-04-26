FROM golang:1.25-alpine

RUN apk add --no-cache git build-base nodejs npm

RUN npm install -g @anthropic-ai/claude-code

WORKDIR /app

RUN go install github.com/air-verse/air@latest

COPY go.mod ./

RUN go mod download

CMD ["air", "-c", ".air.toml"]
