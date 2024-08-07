# Stage 1: Install Node.js and build CSS/JS assets
FROM node:16.20-alpine as asset_builder

WORKDIR /app

# Copy only the assets and package configuration files to minimize layers.
COPY app app
COPY plugins plugins
COPY package.json package-lock.json ./
COPY tailwind.config.js .

RUN npm install
RUN npx tailwindcss -i app/assets/app.css -o ./public/assets/styles.css
RUN npx esbuild app/assets/index.js --bundle --outdir=public/assets

# Stage 2: Build Go application
FROM golang:alpine as builder
ENV CGO_ENABLED=1

WORKDIR /app
COPY . .

# Copy the bundled assets from the asset_builder stage
COPY --from=asset_builder /app/public/assets /app/public/assets

RUN apk add --no-cache --update git build-base
RUN go mod tidy \
    && go build -o app_build cmd/app/main.go

# Stage 3: Final runtime image
FROM golang:alpine as runner

#RUN apk --no-cache add ca-certificates tzdata libc6-compat libgcc libstdc++ make
RUN apk --no-cache add make
WORKDIR /app

COPY --from=builder /app/app_build .
#COPY --from=builder /app/public/assets /app/public/assets
COPY .env.sample .env
COPY Makefile Makefile
COPY app/db/migrations app/db/migrations

VOLUME /app/db


COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh", "./app_build"]
