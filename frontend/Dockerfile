# STAGE 1: React/Vite Builder

FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files 
COPY package*.json ./

# Use Clean Install for strictly deterministic production builds
RUN npm ci

# Copy source code
COPY . .

# Accept build arguments from docker-compose to bake into the static files
ARG VITE_API_BASE_URL
ARG VITE_N8N_WEBHOOK_URL
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
ENV VITE_N8N_WEBHOOK_URL=$VITE_N8N_WEBHOOK_URL

# Compile the React app into pure, static HTML/CSS/JS
RUN npm run build

# STAGE 2: Nginx Production Server

FROM nginx:alpine

COPY --from=builder /app/dist /usr/share/nginx/html

# COPY THE CUSTOM NGINX CONFIG
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]