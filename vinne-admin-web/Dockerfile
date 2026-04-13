# Multi-stage build for React/Vite application
# Stage 1: Build the application
FROM node:20-alpine AS builder

# Set working directory
WORKDIR /app

# Copy package files
COPY package*.json ./

# Install all dependencies (including dev deps for build)
RUN npm ci && \
    npm cache clean --force

# Copy source code
COPY . .

# Set build arguments for environment
ARG BUILD_ENV=production

# Copy the appropriate env file to the correct location based on BUILD_ENV
RUN if [ "$BUILD_ENV" = "development" ] ; then \
      cp .env.development .env.development.local ; \
    elif [ "$BUILD_ENV" = "staging" ] ; then \
      cp .env.staging .env.staging.local ; \
    else \
      cp .env.production .env.production.local ; \
    fi

# Build the application based on environment
RUN if [ "$BUILD_ENV" = "development" ] ; then \
      npx vite build --mode development ; \
    elif [ "$BUILD_ENV" = "staging" ] ; then \
      npx vite build --mode staging ; \
    else \
      npx vite build --mode production ; \
    fi

# Stage 2: Serve with Nginx
FROM nginx:alpine

# Install curl for health checks
RUN apk add --no-cache curl

# Copy custom nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf

# Copy built assets from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# Set correct permissions (nginx user already exists in nginx:alpine)
RUN chown -R nginx:nginx /usr/share/nginx/html && \
    chown -R nginx:nginx /var/cache/nginx && \
    chown -R nginx:nginx /var/log/nginx && \
    touch /var/run/nginx.pid && \
    chown nginx:nginx /var/run/nginx.pid

# Expose port 8080
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Switch to non-root user
USER nginx

# Start nginx
CMD ["nginx", "-g", "daemon off;"]