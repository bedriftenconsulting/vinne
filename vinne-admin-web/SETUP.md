# Spiel Admin Web Portal Setup Guide

Setup instructions for the admin management portal.

## Overview

The Spiel Admin Web Portal is a React-based admin dashboard built with:
- **React 18** with TypeScript
- **Vite** for fast development
- **TanStack Router** for routing
- **TanStack Query** for data fetching
- **Tailwind CSS** for styling
- **Shadcn/ui** for UI components

## Prerequisites

- **Node.js** 18.0 or higher
- **npm** 9.0 or higher
- **Backend services** running (API Gateway on port 4000, Admin Management Service on port 50057)

## Installation

### Step 1: Install Dependencies

```bash
cd spiel-admin-web
npm install
```

### Step 2: Environment Configuration

Create a `.env.localdev` file in the `spiel-admin-web` directory:

```env
VITE_API_BASE_URL=http://localhost:4000/api/v1
```

For different environments:

**Development** (`.env.development`):
```env
VITE_API_BASE_URL=http://localhost:4000/api/v1
```

**Staging** (`.env.staging`):
```env
VITE_API_BASE_URL=https://api-staging.yourdomain.com/api/v1
```

**Production** (`.env.production`):
```env
VITE_API_BASE_URL=https://api.yourdomain.com/api/v1
```

### Step 3: Start Development Server

```bash
npm run dev
```

The application will be available at: **http://localhost:6176**

## Default Admin Credentials

Two admin accounts are available:

1. **Super Admin**
   - Email: `superadmin@randco.com`
   - Password: `Admin@123!`
   - Role: Super Admin (full access)

2. **Custom Admin**
   - Email: `surajmohammedbwoy@gmail.com`
   - Password: `Admin@123!`
   - Role: Super Admin (full access)

**⚠️ IMPORTANT**: Change these passwords immediately after first login in production!

## Project Structure

```
spiel-admin-web/
├── src/
│   ├── components/          # Reusable UI components
│   │   ├── agents/         # Agent management components
│   │   ├── games/          # Game management components
│   │   ├── layouts/        # Layout components
│   │   ├── players/        # Player management components
│   │   ├── ui/             # Base UI components (shadcn)
│   │   └── wallet/         # Wallet management components
│   ├── config/             # Configuration files
│   ├── hooks/              # Custom React hooks
│   ├── lib/                # Utility functions
│   │   ├── api-client.ts  # API client
│   │   ├── api.ts         # API configuration
│   │   └── utils.ts       # Helper functions
│   ├── pages/              # Page components
│   │   ├── AdminPermissions.tsx
│   │   ├── AdminRoles.tsx
│   │   ├── AdminUsers.tsx
│   │   ├── Dashboard.tsx
│   │   ├── Games.tsx
│   │   ├── Draws.tsx
│   │   ├── Players.tsx
│   │   └── ...
│   ├── routes/             # Route definitions
│   ├── services/           # API service functions
│   │   ├── admin.ts       # Admin operations
│   │   ├── agents.ts      # Agent operations
│   │   ├── dashboard.ts   # Dashboard data
│   │   ├── draws.ts       # Draw management
│   │   ├── games.ts       # Game management
│   │   ├── players.ts     # Player management
│   │   └── wallet.ts      # Wallet operations
│   ├── stores/             # State management
│   │   └── auth.ts        # Auth store
│   ├── App.tsx            # Root component
│   └── main.tsx           # Application entry point
├── public/                 # Static assets
├── .env.localdev          # Local environment variables
├── index.html             # HTML template
├── package.json           # Dependencies and scripts
├── tailwind.config.js     # Tailwind configuration
├── tsconfig.json          # TypeScript configuration
└── vite.config.ts         # Vite configuration
```

## Available Scripts

### Development

```bash
npm run dev
```
Starts the development server on port 6176.

### Build

```bash
npm run build
```
Creates an optimized production build.

### Preview

```bash
npm run preview
```
Preview the production build locally.

### Lint

```bash
npm run lint
```
Run ESLint to check code quality.

## Features

### Dashboard

- **System Overview**: Key metrics and statistics
- **Recent Activity**: Latest transactions and events
- **Quick Actions**: Common administrative tasks
- **Charts & Analytics**: Visual data representation

### Game Management

- **Create Games**: Configure new lottery games
  - Game name and description
  - Ticket price and prize structure
  - Draw schedule and frequency
  - Number selection rules
- **Edit Games**: Update game configurations
- **Activate/Deactivate**: Control game availability
- **View Game Stats**: Performance metrics

### Draw Management

- **Schedule Draws**: Set up upcoming draws
- **Execute Draws**: Run draw process
- **View Results**: Check winning numbers
- **Draw History**: Past draw records
- **Cancel Draws**: Cancel scheduled draws

### Player Management

- **View Players**: List all registered players
- **Player Details**: View player profile and activity
- **Player Transactions**: Financial history
- **Player Tickets**: Purchased tickets
- **Account Status**: Activate/deactivate accounts
- **Search & Filter**: Find specific players

### Agent Management

- **View Agents**: List all agents/retailers
- **Agent Profile**: Detailed agent information
- **Agent Performance**: Sales and commission data
- **POS Terminals**: Terminal management
- **Commission Settings**: Configure commission rates

### Wallet Management

- **Credit Wallets**: Add funds to player wallets
- **Transaction History**: View all transactions
- **Pending Transactions**: Review pending operations
- **Refunds**: Process refund requests
- **Balance Adjustments**: Manual balance corrections

### Ticket Management

- **View Tickets**: All tickets in system
- **Ticket Details**: Individual ticket information
- **Validate Tickets**: Check winning tickets
- **Cancel Tickets**: Cancel tickets if needed
- **Ticket Reports**: Sales and winning reports

### Admin User Management

- **Create Admins**: Add new admin users
- **Manage Roles**: Assign roles and permissions
- **User Permissions**: Fine-grained access control
- **Audit Logs**: Track admin activities
- **Session Management**: Active sessions

### Reports & Analytics

- **Sales Reports**: Revenue and sales data
- **Player Reports**: User engagement metrics
- **Game Performance**: Game-specific analytics
- **Financial Reports**: Transaction summaries
- **Export Data**: Download reports as CSV/PDF

### Audit Logs

- **Activity Tracking**: All admin actions logged
- **Filter Logs**: Search by user, action, date
- **Export Logs**: Download audit trail
- **Security Monitoring**: Suspicious activity alerts

## Admin Roles & Permissions

### Super Admin
- Full system access
- User management
- System configuration
- All CRUD operations

### Admin
- Most administrative functions
- Cannot modify system settings
- Cannot manage super admins

### Manager
- Operational tasks
- Game and draw management
- Player support
- Reports access

### Support
- Player assistance
- Ticket validation
- Read-only access to most data
- Limited transaction operations

### Viewer
- Read-only access
- View reports and analytics
- No modification permissions

## Configuration

### API Integration

```typescript
// src/lib/api-client.ts
import axios from 'axios';

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add auth token to requests
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('admin_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});
```

### Authentication Flow

1. Admin enters email and password
2. API validates credentials
3. Returns access token and refresh token
4. Tokens stored in localStorage
5. Token included in all subsequent requests
6. Automatic token refresh on expiry
7. Redirect to login on authentication failure

### Protected Routes

```typescript
// src/components/ProtectedRoute.tsx
export function ProtectedRoute({ children }) {
  const { isAuthenticated } = useAuthStore();
  
  if (!isAuthenticated) {
    return <Navigate to="/login" />;
  }
  
  return children;
}
```

## Common Tasks

### Creating a New Game

1. Navigate to **Games** page
2. Click **"Create Game"** button
3. Fill in game details:
   - Name and description
   - Ticket price
   - Prize structure
   - Draw schedule
   - Number selection rules
4. Click **"Save"**
5. Activate game when ready

### Scheduling a Draw

1. Navigate to **Draws** page
2. Click **"Schedule Draw"**
3. Select game
4. Set draw date and time
5. Configure draw parameters
6. Click **"Schedule"**

### Managing Player Wallet

1. Navigate to **Players** page
2. Search for player
3. Click on player name
4. Go to **Wallet** tab
5. Click **"Credit Wallet"** or **"Adjust Balance"**
6. Enter amount and reason
7. Confirm transaction

### Creating Admin User

1. Navigate to **Admin Users** page
2. Click **"Create Admin"**
3. Enter user details:
   - Email
   - Username
   - First and last name
   - Initial password
4. Assign role(s)
5. Click **"Create"**
6. User receives email with credentials

## Common Issues

### Issue: Cannot log in

**Solutions**:
- Verify Admin Management Service is running (port 50057)
- Check API Gateway is running (port 4000)
- Ensure credentials are correct
- Check browser console for errors
- Verify backend logs for authentication errors

### Issue: 500 Internal Server Error on login

**Solution**: Check that admin user exists in database
```bash
docker exec -it spiel-microservices-service-admin-management-db-1 psql -U admin_mgmt -d admin_management -c "SELECT username, email FROM admin_users;"
```

### Issue: Unauthorized errors after login

**Solution**: Token may be expired or invalid
- Clear localStorage
- Log in again
- Check JWT_SECRET matches across services

### Issue: Cannot create games

**Solution**: Ensure Game Service is running (port 50053)

### Issue: Draws not showing

**Solution**: Ensure Draw Service is running (port 50060)

## Security Best Practices

- Change default admin passwords immediately
- Use strong passwords (min 12 characters, mixed case, numbers, symbols)
- Enable MFA for admin accounts (if implemented)
- Regularly review audit logs
- Limit admin user creation
- Use role-based access control
- Log out when not in use
- Use HTTPS in production
- Implement IP whitelisting for admin access
- Regular security audits

## Performance Tips

- Use pagination for large data sets
- Implement search and filters
- Cache frequently accessed data
- Lazy load components
- Optimize images
- Use production build for deployment

## Deployment

### Build for Production

```bash
npm run build
```

Output will be in `dist/` directory.

### Deploy with Nginx

```nginx
server {
    listen 80;
    server_name admin.yourdomain.com;
    
    root /var/www/admin-web/dist;
    index index.html;
    
    location / {
        try_files $uri $uri/ /index.html;
    }
    
    location /api {
        proxy_pass http://api-gateway:4000;
    }
}
```

### Deploy with Docker

```dockerfile
FROM node:18-alpine as build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
```

### Environment Variables for Production

```env
VITE_API_BASE_URL=https://api.yourdomain.com/api/v1
```

## Monitoring & Logging

### Error Tracking

Integrate Sentry for error tracking:

```typescript
import * as Sentry from "@sentry/react";

Sentry.init({
  dsn: "your-sentry-dsn",
  environment: import.meta.env.MODE,
});
```

### Analytics

Track admin actions:

```typescript
// Track page views
analytics.page('Dashboard');

// Track events
analytics.track('Game Created', {
  gameId: game.id,
  gameName: game.name,
});
```

## Testing Checklist

- [ ] Log in with admin credentials
- [ ] View dashboard metrics
- [ ] Create a new game
- [ ] Schedule a draw
- [ ] View player list
- [ ] Credit player wallet
- [ ] View transaction history
- [ ] Create admin user
- [ ] Assign roles and permissions
- [ ] View audit logs
- [ ] Export reports
- [ ] Log out

## Support

For issues:
- Check backend service logs
- Review browser console
- Verify environment variables
- Ensure all services are running
- Check database connectivity

## Additional Resources

- [React Documentation](https://react.dev/)
- [Vite Documentation](https://vitejs.dev/)
- [TanStack Router](https://tanstack.com/router)
- [TanStack Query](https://tanstack.com/query)
- [Tailwind CSS](https://tailwindcss.com/)
- [Shadcn/ui](https://ui.shadcn.com/)
