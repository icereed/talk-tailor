# Stage 1: Build React frontend
FROM node:16 AS build-stage
WORKDIR /app
COPY client/package*.json ./
RUN npm install
COPY client/ ./
RUN npm run build

# Stage 2: Build Go backend
FROM golang:1.23-alpine AS go-build-stage
ENV CGO_ENABLED=0
WORKDIR /app
RUN apk add --no-cache ffmpeg
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test ./... && go build -o talk-tailor .

# Stage 3: Run the app
FROM alpine:3

WORKDIR /app

# install ffmpeg
RUN apk add --no-cache ffmpeg
COPY --from=build-stage /app/dist/ ./client/dist
COPY --from=go-build-stage /app/talk-tailor  /app/talk-tailor

ENTRYPOINT [ "/app/talk-tailor" ]