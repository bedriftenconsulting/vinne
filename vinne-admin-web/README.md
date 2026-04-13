# Admin Web Portal

Administrative dashboard for managing the Spiel Lottery platform.

**URL**: http://localhost:6176

---

## Dashboard Preview

![Admin Agent Dashboard](https://imgur.com/3WLXArc.png)

---

## Features

### User Management
- Create and manage admin users
- Role-based access control (RBAC)
- Permission management
- Audit logging

### Agent Management
- Create new agents
- View agent profiles
- Monitor agent performance
- Manage agent status

### Retailer Oversight
- View all retailers
- Monitor retailer sales
- Track retailer performance
- Manage retailer status

### Financial Operations
- Transaction monitoring
- Wallet management
- Commission tracking
- Financial reports

### System Management
- Game configuration
- Draw management
- Ticket operations
- System analytics

---

## Quick Start

### Prerequisites
- Node.js 18+
- npm or yarn
- Backend services running (see [LOCAL_SETUP.md](../LOCAL_SETUP.md))

### Installation

```bash
cd spiel-admin-web
npm install
```

### Configuration

Create `.env.localdev` file:

```env
VITE_API_URL=http://localhost:4000/api/v1
VITE_APP_URL=http://localhost:6176
VITE_ENVIRONMENT=development
```

### Run Development Server

```bash
npm run dev
```

Access at: http://localhost:6176

---

## Default Credentials

### Super Admin
```
Email: superadmin@randco.com
Password: Admin@123!
```

### Alternative Admin
```
Email: surajmohammedbwoy@gmail.com
Password: Admin@123!
```

⚠️ **Change these passwords in production!**

---

## Key Sections

### Dashboard
- Overview metrics
- Recent activity
- System health
- Quick actions

### Agents
- Agent list with search/filter
- Create new agent
- View agent details
- Update agent status
- Performance metrics

### Retailers
- Retailer list
- Retailer details
- Sales analytics
- Commission reports

### Players
- Player accounts
- Player activity
- Transaction history
- Account management

### Transactions
- All transactions
- Filter by type/status
- Transaction details
- Financial reports

### Games
- Game configuration
- Game rules
- Prize structures
- Game status

### Draws
- Schedule draws
- View results
- Winner management
- Draw history

### Audit Logs
- System activity
- User actions
- Security events
- Compliance tracking

---

## Common Tasks

### Creating an Agent

1. Navigate to **Agents** section
2. Click **Create Agent** button
3. Fill in required fields:
   - Name
   - Email
   - Phone (format: 023456789)
   - Address
   - Commission percentage (optional)
4. Click **Submit**
5. Agent code is auto-generated (e.g., `1001`)
6. Default password: `123456`

### Managing Permissions

1. Navigate to **Admin Users** → **Roles**
2. Select or create a role
3. Assign permissions
4. Save changes

### Viewing Audit Logs

1. Navigate to **Audit Logs**
2. Filter by:
   - Date range
   - User
   - Action type
   - Entity
3. View detailed log entries

---

## Tech Stack

- **React** 18
- **TypeScript**
- **Vite**
- **TailwindCSS**
- **TanStack Router**
- **TanStack Query**
- **Zustand** (State management)
- **Axios** (HTTP client)

---

## Project Structure

```
spiel-admin-web/
├── src/
│   ├── components/     # Reusable UI components
│   ├── pages/          # Page components
│   ├── services/       # API services
│   ├── stores/         # State management
│   ├── lib/            # Utilities
│   └── routes/         # Route definitions
├── public/             # Static assets
└── .env.localdev       # Environment config
```

---

## Troubleshooting

### Issue: Login fails with 500 error

**Solution**: Verify Admin Management Service is running on port 50057

```bash
# Check if service is running
curl http://localhost:50057/health
```

### Issue: "Invalid or expired token"

**Solution**: Log out and log in again to get a fresh JWT token

### Issue: Agent creation fails

**Solution**: 
1. Check agent-management database has tables
2. Verify phone number format (no country code)
3. Check backend service logs

---

## Related Documentation

- [Local Development Setup](../LOCAL_SETUP.md)
- [Microservices Setup](../spiel-microservices/SETUP.md)
- [Agent Portal](../spiel-agent-web/README.md)
- [Player Website](../spiel-website/README.md)

---

## License

Proprietary - Spiel
