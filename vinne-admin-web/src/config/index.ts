// Application configuration based on environment variables

export const config = {
  api: {
    baseUrl: import.meta.env.VITE_API_URL || 'http://localhost:4000/api/v1',
  },
  app: {
    url: import.meta.env.VITE_APP_URL || 'http://localhost:5173',
    environment: import.meta.env.VITE_ENVIRONMENT || 'development',
  },
  debug: {
    enabled: import.meta.env.VITE_ENABLE_DEBUG === 'true',
    logLevel: import.meta.env.VITE_LOG_LEVEL || 'info',
  },
  features: {
    // Feature flags can be added here
    mockData: import.meta.env.VITE_ENVIRONMENT === 'development',
  },
} as const

// Type for the config
export type AppConfig = typeof config

// Helper to check environment
export const isDevelopment = () => config.app.environment === 'development'
export const isStaging = () => config.app.environment === 'staging'
export const isProduction = () => config.app.environment === 'production'

// Export environment check
export const environment = config.app.environment
