import { useState } from 'react'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Search,
  Download,
  Filter,
  RefreshCw,
  Plus,
  Monitor,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Clock,
  Settings,
  MoreHorizontal,
  Wifi,
  WifiOff,
  MapPin,
  Activity,
  RotateCcw,
  Upload,
  Eye,
  Edit,
  Trash2,
} from 'lucide-react'

// Terminal data interface
interface Terminal {
  id: string
  imei: string
  model: string
  manufacturer: string
  status: string
  retailer: string
  retailerId: string
  agent: string
  agentCode: string
  location: string
  assignedDate: string
  lastSync: string
  lastTransaction: string
  appVersion: string
  osVersion: string
  networkOperator: string
  signalStrength: number
  batteryLevel: number
  isOnline: boolean
  totalTransactions: number
  totalValue: number
  dailyTransactions: number
  dailyValue: number
  configVersion: string
  lastConfigUpdate: string
  healthStatus: string
  diagnostics: {
    storage: { used: number; total: number; unit: string }
    memory: { used: number; total: number; unit: string }
    temperature: number
    connectivity: string
  }
}

// Comprehensive mock POS terminal data based on PRD requirements
const generateMockTerminals = () => {
  const baseTerminals = [
    {
      id: 'POS-2025-000001',
      imei: '861234567890123',
      model: 'Sunmi V2 Pro',
      manufacturer: 'Sunmi',
      status: 'ACTIVE',
      retailer: 'Accra Central Shop',
      retailerId: '00002345',
      agent: 'Kwame Asante',
      agentCode: 'AGT-1001',
      location: 'Accra Central Market, Accra',
      assignedDate: '2025-01-02T08:30:00Z',
      lastSync: '2025-01-05T14:25:00Z',
      lastTransaction: '2025-01-05T13:45:00Z',
      appVersion: '2.1.5',
      osVersion: 'Android 12',
      networkOperator: 'MTN',
      signalStrength: 85,
      batteryLevel: 78,
      isOnline: true,
      totalTransactions: 342,
      totalValue: 85600,
      dailyTransactions: 23,
      dailyValue: 5480,
      configVersion: 'CFG-v1.2.3',
      lastConfigUpdate: '2024-12-28T10:15:00Z',
      healthStatus: 'HEALTHY',
      diagnostics: {
        storage: { used: 2.3, total: 8.0, unit: 'GB' },
        memory: { used: 1.8, total: 3.0, unit: 'GB' },
        temperature: 42,
        connectivity: 'GOOD',
      },
    },
    {
      id: 'POS-2025-000002',
      imei: '861234567890124',
      model: 'Ingenico Move/5000',
      manufacturer: 'Ingenico',
      status: 'INACTIVE',
      retailer: 'Kumasi Junction',
      retailerId: '00002346',
      agent: 'Ama Serwaa',
      agentCode: 'AGT-1002',
      location: 'Kejetia Market, Kumasi',
      assignedDate: '2024-12-15T11:20:00Z',
      lastSync: '2025-01-03T16:10:00Z',
      lastTransaction: '2025-01-03T15:22:00Z',
      appVersion: '2.1.3',
      osVersion: 'Android 11',
      networkOperator: 'Vodafone',
      signalStrength: 42,
      batteryLevel: 15,
      isOnline: false,
      totalTransactions: 156,
      totalValue: 42300,
      dailyTransactions: 0,
      dailyValue: 0,
      configVersion: 'CFG-v1.2.1',
      lastConfigUpdate: '2024-12-20T14:30:00Z',
      healthStatus: 'WARNING',
      diagnostics: {
        storage: { used: 3.1, total: 8.0, unit: 'GB' },
        memory: { used: 2.6, total: 3.0, unit: 'GB' },
        temperature: 58,
        connectivity: 'POOR',
      },
    },
    {
      id: 'POS-2025-000003',
      imei: '861234567890125',
      model: 'PAX A920 Pro',
      manufacturer: 'PAX',
      status: 'MAINTENANCE',
      retailer: 'Tamale Market',
      retailerId: '00002347',
      agent: 'Kofi Mensah',
      agentCode: 'AGT-1003',
      location: 'Central Market, Tamale',
      assignedDate: '2024-11-22T09:45:00Z',
      lastSync: '2025-01-05T08:15:00Z',
      lastTransaction: '2025-01-04T17:30:00Z',
      appVersion: '2.1.5',
      osVersion: 'Android 12',
      networkOperator: 'AirtelTigo',
      signalStrength: 68,
      batteryLevel: 95,
      isOnline: true,
      totalTransactions: 728,
      totalValue: 156800,
      dailyTransactions: 8,
      dailyValue: 2100,
      configVersion: 'CFG-v1.2.3',
      lastConfigUpdate: '2024-12-28T10:15:00Z',
      healthStatus: 'MAINTENANCE',
      diagnostics: {
        storage: { used: 4.2, total: 16.0, unit: 'GB' },
        memory: { used: 2.1, total: 4.0, unit: 'GB' },
        temperature: 38,
        connectivity: 'EXCELLENT',
      },
    },
    {
      id: 'POS-2025-000004',
      imei: '861234567890126',
      model: 'Sunmi V2 Pro',
      manufacturer: 'Sunmi',
      status: 'FAULTY',
      retailer: 'Cape Coast Plaza',
      retailerId: '00002348',
      agent: 'Akosua Osei',
      agentCode: 'AGT-1004',
      location: 'Cape Coast Central, Cape Coast',
      assignedDate: '2024-10-18T13:25:00Z',
      lastSync: '2025-01-02T11:45:00Z',
      lastTransaction: '2025-01-02T10:15:00Z',
      appVersion: '2.1.2',
      osVersion: 'Android 11',
      networkOperator: 'MTN',
      signalStrength: 0,
      batteryLevel: 0,
      isOnline: false,
      totalTransactions: 89,
      totalValue: 21400,
      dailyTransactions: 0,
      dailyValue: 0,
      configVersion: 'CFG-v1.1.8',
      lastConfigUpdate: '2024-11-15T16:20:00Z',
      healthStatus: 'CRITICAL',
      diagnostics: {
        storage: { used: 0, total: 8.0, unit: 'GB' },
        memory: { used: 0, total: 3.0, unit: 'GB' },
        temperature: 0,
        connectivity: 'NONE',
      },
    },
  ]

  // Generate additional terminals for pagination testing
  const additionalTerminals = []
  const retailers = [
    'Ho Central',
    'Takoradi Mall',
    'Wa Market',
    'Bolgatanga Station',
    'Techiman Junction',
  ]
  const agents = ['Yaw Boateng', 'Efua Asante', 'Kwaku Owusu', 'Adwoa Mensah', 'Kofi Asare']
  const agentCodes = ['AGT-1005', 'AGT-1006', 'AGT-1007', 'AGT-1008', 'AGT-1009']
  const models = [
    'Sunmi V2 Pro',
    'Ingenico Move/5000',
    'PAX A920 Pro',
    'Verifone V400m',
    'Newland NQuire 1000',
  ]
  const manufacturers = ['Sunmi', 'Ingenico', 'PAX', 'Verifone', 'Newland']
  const statuses = ['ACTIVE', 'INACTIVE', 'MAINTENANCE', 'FAULTY', 'DECOMMISSIONED']
  const networks = ['MTN', 'Vodafone', 'AirtelTigo']
  const locations = [
    'Ho Central Market',
    'Takoradi Harbour Area',
    'Wa Central Market',
    'Bolgatanga Main Station',
    'Techiman Junction',
  ]

  for (let i = 5; i <= 120; i++) {
    const randomRetailer = retailers[Math.floor(Math.random() * retailers.length)]
    const randomAgent = agents[Math.floor(Math.random() * agents.length)]
    const randomAgentCode = agentCodes[Math.floor(Math.random() * agentCodes.length)]
    const randomModel = models[Math.floor(Math.random() * models.length)]
    const randomManufacturer = manufacturers[models.indexOf(randomModel)]
    const randomStatus = statuses[Math.floor(Math.random() * statuses.length)]
    const randomNetwork = networks[Math.floor(Math.random() * networks.length)]
    const randomLocation = locations[Math.floor(Math.random() * locations.length)]

    const isOnline = randomStatus === 'ACTIVE' ? Math.random() > 0.2 : Math.random() > 0.8
    const signalStrength = isOnline ? Math.floor(Math.random() * 60) + 40 : 0
    const batteryLevel = randomStatus === 'FAULTY' ? 0 : Math.floor(Math.random() * 100)

    // Generate random timestamps
    const assignedDate = new Date(Date.now() - Math.random() * 180 * 24 * 60 * 60 * 1000) // Last 6 months
    const lastSync = new Date(Date.now() - Math.random() * 7 * 24 * 60 * 60 * 1000) // Last week
    const lastTransaction = new Date(lastSync.getTime() - Math.random() * 24 * 60 * 60 * 1000) // Before last sync

    additionalTerminals.push({
      id: `POS-2025-${i.toString().padStart(6, '0')}`,
      imei: `86123456789${(123 + i).toString().padStart(4, '0')}`,
      model: randomModel,
      manufacturer: randomManufacturer,
      status: randomStatus,
      retailer: randomRetailer,
      retailerId: `0000${(2345 + i).toString()}`,
      agent: randomAgent,
      agentCode: randomAgentCode,
      location: randomLocation,
      assignedDate: assignedDate.toISOString(),
      lastSync: lastSync.toISOString(),
      lastTransaction: lastTransaction.toISOString(),
      appVersion: `2.1.${Math.floor(Math.random() * 6)}`,
      osVersion: `Android ${Math.random() > 0.5 ? '12' : '11'}`,
      networkOperator: randomNetwork,
      signalStrength,
      batteryLevel,
      isOnline,
      totalTransactions: Math.floor(Math.random() * 1000),
      totalValue: Math.floor(Math.random() * 200000),
      dailyTransactions: isOnline ? Math.floor(Math.random() * 50) : 0,
      dailyValue: isOnline ? Math.floor(Math.random() * 10000) : 0,
      configVersion: `CFG-v1.${Math.floor(Math.random() * 3)}.${Math.floor(Math.random() * 5)}`,
      lastConfigUpdate: new Date(
        Date.now() - Math.random() * 30 * 24 * 60 * 60 * 1000
      ).toISOString(),
      healthStatus:
        randomStatus === 'ACTIVE' ? 'HEALTHY' : randomStatus === 'FAULTY' ? 'CRITICAL' : 'WARNING',
      diagnostics: {
        storage: { used: Math.round(Math.random() * 8 * 10) / 10, total: 8.0, unit: 'GB' },
        memory: { used: Math.round(Math.random() * 3 * 10) / 10, total: 3.0, unit: 'GB' },
        temperature: Math.floor(Math.random() * 40) + 25,
        connectivity: isOnline
          ? signalStrength > 70
            ? 'EXCELLENT'
            : signalStrength > 50
              ? 'GOOD'
              : 'POOR'
          : 'NONE',
      },
    })
  }

  return [...baseTerminals, ...additionalTerminals]
}

const mockTerminals = generateMockTerminals()

const mockSummary = {
  totalTerminals: 120,
  activeTerminals: 78,
  inactiveTerminals: 25,
  maintenanceTerminals: 12,
  faultyTerminals: 4,
  decommissionedTerminals: 1,
  onlineTerminals: 85,
  offlineTerminals: 35,
  avgDailyTransactions: 18.5,
  totalDailyValue: 450000,
  outdatedVersions: 15,
  healthyTerminals: 95,
  warningTerminals: 20,
  criticalTerminals: 5,
}

export default function PosTerminals() {
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [healthFilter, setHealthFilter] = useState<string>('all')
  const [onlineFilter, setOnlineFilter] = useState<string>('all')
  const [currentPage, setCurrentPage] = useState(1)
  const [selectedTerminal, setSelectedTerminal] = useState<Terminal | null>(null)
  const [showDetailsModal, setShowDetailsModal] = useState(false)
  const [showAssignModal, setShowAssignModal] = useState(false)
  const itemsPerPage = 15

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('en-GH', {
      style: 'currency',
      currency: 'GHS',
    }).format(amount)
  }

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-GB', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const getStatusBadgeVariant = (status: string) => {
    switch (status) {
      case 'ACTIVE':
        return 'default'
      case 'INACTIVE':
        return 'secondary'
      case 'MAINTENANCE':
        return 'outline'
      case 'FAULTY':
        return 'destructive'
      case 'DECOMMISSIONED':
        return 'secondary'
      default:
        return 'outline'
    }
  }

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'ACTIVE':
        return <CheckCircle className="h-4 w-4 text-green-500" />
      case 'INACTIVE':
        return <Clock className="h-4 w-4 text-yellow-500" />
      case 'MAINTENANCE':
        return <Settings className="h-4 w-4 text-blue-500" />
      case 'FAULTY':
        return <XCircle className="h-4 w-4 text-red-500" />
      case 'DECOMMISSIONED':
        return <AlertTriangle className="h-4 w-4 text-gray-500" />
      default:
        return <Clock className="h-4 w-4 text-gray-500" />
    }
  }

  const getHealthBadgeVariant = (health: string) => {
    switch (health) {
      case 'HEALTHY':
        return 'default'
      case 'WARNING':
        return 'secondary'
      case 'CRITICAL':
        return 'destructive'
      case 'MAINTENANCE':
        return 'outline'
      default:
        return 'outline'
    }
  }

  const filteredTerminals = mockTerminals.filter(terminal => {
    const matchesSearch =
      terminal.id.toLowerCase().includes(search.toLowerCase()) ||
      terminal.imei.toLowerCase().includes(search.toLowerCase()) ||
      terminal.retailer.toLowerCase().includes(search.toLowerCase()) ||
      terminal.agent.toLowerCase().includes(search.toLowerCase()) ||
      terminal.location.toLowerCase().includes(search.toLowerCase())

    const matchesStatus = statusFilter === 'all' || terminal.status === statusFilter
    const matchesHealth = healthFilter === 'all' || terminal.healthStatus === healthFilter
    const matchesOnline =
      onlineFilter === 'all' ||
      (onlineFilter === 'online' && terminal.isOnline) ||
      (onlineFilter === 'offline' && !terminal.isOnline)

    return matchesSearch && matchesStatus && matchesHealth && matchesOnline
  })

  // Sort terminals by last sync (most recent first)
  const sortedTerminals = [...filteredTerminals].sort((a, b) => {
    const dateA = new Date(a.lastSync)
    const dateB = new Date(b.lastSync)
    return dateB.getTime() - dateA.getTime()
  })

  // Pagination
  const totalPages = Math.ceil(sortedTerminals.length / itemsPerPage)
  const startIndex = (currentPage - 1) * itemsPerPage
  const endIndex = startIndex + itemsPerPage
  const paginatedTerminals = sortedTerminals.slice(startIndex, endIndex)

  const handleTerminalAction = (action: string, terminal: Terminal) => {
    console.log(`${action} action for terminal:`, terminal.id)
    // Here you would implement the actual actions
  }

  return (
    <div className="p-3 sm:p-4 md:p-6 space-y-3 sm:space-y-4 md:space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div className="space-y-1 sm:space-y-2 min-w-0 flex-1">
          <h1 className="text-xl sm:text-2xl md:text-3xl font-bold tracking-tight">
            POS Terminals
          </h1>
          <p className="text-xs sm:text-sm text-muted-foreground">
            Monitor and manage POS terminal devices across all retail locations
          </p>
        </div>
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 sm:gap-3 w-full sm:w-auto">
          <Button variant="outline" size="sm" className="w-full sm:w-auto">
            <RefreshCw className="w-3 sm:w-4 h-3 sm:h-4 mr-2" />
            <span className="hidden sm:inline">Sync All</span>
            <span className="sm:hidden">Sync</span>
          </Button>
          <Button variant="outline" size="sm" className="w-full sm:w-auto">
            <Download className="w-3 sm:w-4 h-3 sm:h-4 mr-2" />
            Export
          </Button>
          <Dialog open={showAssignModal} onOpenChange={setShowAssignModal}>
            <DialogTrigger asChild>
              <Button size="sm" className="w-full sm:w-auto">
                <Plus className="w-3 sm:w-4 h-3 sm:h-4 mr-2" />
                <span className="hidden sm:inline">Assign Terminal</span>
                <span className="sm:hidden">Assign</span>
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Assign New Terminal</DialogTitle>
                <DialogDescription>
                  Register and assign a new POS terminal to a retailer
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4">
                <Input placeholder="Terminal ID (e.g., POS-2025-000121)" />
                <Input placeholder="IMEI Number" />
                <Select>
                  <SelectTrigger>
                    <SelectValue placeholder="Select Retailer" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="retail1">Accra Central Shop</SelectItem>
                    <SelectItem value="retail2">Kumasi Junction</SelectItem>
                    <SelectItem value="retail3">Tamale Market</SelectItem>
                  </SelectContent>
                </Select>
                <Select>
                  <SelectTrigger>
                    <SelectValue placeholder="Select Terminal Model" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="sunmi">Sunmi V2 Pro</SelectItem>
                    <SelectItem value="ingenico">Ingenico Move/5000</SelectItem>
                    <SelectItem value="pax">PAX A920 Pro</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setShowAssignModal(false)}>
                  Cancel
                </Button>
                <Button onClick={() => setShowAssignModal(false)}>Assign Terminal</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </div>

      {/* Summary Cards */}
      <div className="space-y-3 sm:space-y-4">
        {/* First Row */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 sm:gap-4">
          <Card>
            <CardContent className="p-3 sm:p-4">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                    Total Terminals
                  </p>
                  <p className="text-xl sm:text-2xl font-bold">{mockSummary.totalTerminals}</p>
                  <p className="text-xs text-muted-foreground">Registered devices</p>
                </div>
                <Monitor className="w-6 sm:w-8 h-6 sm:h-8 text-muted-foreground flex-shrink-0 ml-2" />
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-3 sm:p-4">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                    Active
                  </p>
                  <p className="text-xl sm:text-2xl font-bold text-green-600">
                    {mockSummary.activeTerminals}
                  </p>
                  <p className="text-xs text-muted-foreground">Operational</p>
                </div>
                <CheckCircle className="w-6 sm:w-8 h-6 sm:h-8 text-green-500 flex-shrink-0 ml-2" />
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-3 sm:p-4">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                    Online
                  </p>
                  <p className="text-xl sm:text-2xl font-bold text-blue-600">
                    {mockSummary.onlineTerminals}
                  </p>
                  <p className="text-xs text-muted-foreground">Connected now</p>
                </div>
                <Wifi className="w-6 sm:w-8 h-6 sm:h-8 text-blue-500 flex-shrink-0 ml-2" />
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-3 sm:p-4">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                    Daily Volume
                  </p>
                  <p className="text-xl sm:text-2xl font-bold">
                    {formatCurrency(mockSummary.totalDailyValue).replace('GH₵', 'GH₵')}
                  </p>
                  <p className="text-xs text-muted-foreground">Today's transactions</p>
                </div>
                <Activity className="w-6 sm:w-8 h-6 sm:h-8 text-muted-foreground flex-shrink-0 ml-2" />
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Second Row */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 sm:gap-4">
          <Card>
            <CardContent className="p-3 sm:p-4">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                    Offline
                  </p>
                  <p className="text-xl sm:text-2xl font-bold text-red-600">
                    {mockSummary.offlineTerminals}
                  </p>
                  <p className="text-xs text-muted-foreground">Need attention</p>
                </div>
                <WifiOff className="w-6 sm:w-8 h-6 sm:h-8 text-red-500 flex-shrink-0 ml-2" />
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-3 sm:p-4">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                    Faulty
                  </p>
                  <p className="text-xl sm:text-2xl font-bold text-orange-600">
                    {mockSummary.faultyTerminals}
                  </p>
                  <p className="text-xs text-muted-foreground">Need repair</p>
                </div>
                <XCircle className="w-6 sm:w-8 h-6 sm:h-8 text-orange-500 flex-shrink-0 ml-2" />
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-3 sm:p-4">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                    Maintenance
                  </p>
                  <p className="text-xl sm:text-2xl font-bold">
                    {mockSummary.maintenanceTerminals}
                  </p>
                  <p className="text-xs text-muted-foreground">Scheduled</p>
                </div>
                <Settings className="w-6 sm:w-8 h-6 sm:h-8 text-muted-foreground flex-shrink-0 ml-2" />
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-3 sm:p-4">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                    Outdated
                  </p>
                  <p className="text-xl sm:text-2xl font-bold text-yellow-600">
                    {mockSummary.outdatedVersions}
                  </p>
                  <p className="text-xs text-muted-foreground">Need update</p>
                </div>
                <Upload className="w-6 sm:w-8 h-6 sm:h-8 text-yellow-500 flex-shrink-0 ml-2" />
              </div>
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Filters and Search */}
      <Card>
        <CardHeader>
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 sm:gap-3 md:gap-4 flex-wrap">
            <div className="flex items-center gap-2 flex-1 min-w-0">
              <Search className="h-3 sm:h-4 w-3 sm:w-4 shrink-0" />
              <Input
                placeholder="Search terminals, retailers, agents..."
                value={search}
                onChange={e => setSearch(e.target.value)}
                className="w-full sm:max-w-sm text-xs sm:text-sm"
              />
            </div>

            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="w-full sm:w-36 md:w-40 text-xs sm:text-sm">
                <SelectValue placeholder="Status" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Status</SelectItem>
                <SelectItem value="ACTIVE">Active</SelectItem>
                <SelectItem value="INACTIVE">Inactive</SelectItem>
                <SelectItem value="MAINTENANCE">Maintenance</SelectItem>
                <SelectItem value="FAULTY">Faulty</SelectItem>
                <SelectItem value="DECOMMISSIONED">Decommissioned</SelectItem>
              </SelectContent>
            </Select>

            <Select value={onlineFilter} onValueChange={setOnlineFilter}>
              <SelectTrigger className="w-full sm:w-32 md:w-36 text-xs sm:text-sm">
                <SelectValue placeholder="Connection" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All</SelectItem>
                <SelectItem value="online">Online</SelectItem>
                <SelectItem value="offline">Offline</SelectItem>
              </SelectContent>
            </Select>

            <Select value={healthFilter} onValueChange={setHealthFilter}>
              <SelectTrigger className="w-full sm:w-32 md:w-36 text-xs sm:text-sm">
                <SelectValue placeholder="Health" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Health</SelectItem>
                <SelectItem value="HEALTHY">Healthy</SelectItem>
                <SelectItem value="WARNING">Warning</SelectItem>
                <SelectItem value="CRITICAL">Critical</SelectItem>
                <SelectItem value="MAINTENANCE">Maintenance</SelectItem>
              </SelectContent>
            </Select>

            <Button variant="outline" size="sm" className="w-full sm:w-auto">
              <Filter className="h-3 sm:h-4 w-3 sm:w-4 mr-2" />
              <span className="hidden sm:inline">Advanced Filters</span>
              <span className="sm:hidden">Filters</span>
            </Button>
          </div>
        </CardHeader>

        <CardContent>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="text-xs sm:text-sm">Terminal ID</TableHead>
                  <TableHead className="text-xs sm:text-sm">Retailer</TableHead>
                  <TableHead className="text-xs sm:text-sm">Status</TableHead>
                  <TableHead className="text-xs sm:text-sm">Connection</TableHead>
                  <TableHead className="text-xs sm:text-sm">Health</TableHead>
                  <TableHead className="text-xs sm:text-sm">Last Sync</TableHead>
                  <TableHead className="text-xs sm:text-sm">Daily Stats</TableHead>
                  <TableHead className="text-xs sm:text-sm">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {paginatedTerminals.map(terminal => (
                  <TableRow key={terminal.id}>
                    <TableCell className="font-medium">
                      <div className="space-y-1">
                        <div className="font-mono text-xs sm:text-sm font-bold">{terminal.id}</div>
                        <div className="text-xs text-muted-foreground">
                          {terminal.model} • {terminal.imei}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="space-y-1">
                        <div className="font-medium text-xs sm:text-sm">{terminal.retailer}</div>
                        <div className="text-xs text-muted-foreground">{terminal.agent}</div>
                        <div className="flex items-center text-xs text-muted-foreground">
                          <MapPin className="h-3 w-3 mr-1" />
                          {terminal.location}
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center space-x-2">
                        {getStatusIcon(terminal.status)}
                        <Badge variant={getStatusBadgeVariant(terminal.status)} className="text-xs">
                          {terminal.status}
                        </Badge>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center space-x-2">
                        {terminal.isOnline ? (
                          <Wifi className="h-3 sm:h-4 w-3 sm:w-4 text-green-500" />
                        ) : (
                          <WifiOff className="h-3 sm:h-4 w-3 sm:w-4 text-red-500" />
                        )}
                        <div className="text-xs sm:text-sm">
                          {terminal.isOnline ? 'Online' : 'Offline'}
                        </div>
                      </div>
                      {terminal.isOnline && (
                        <div className="text-xs text-muted-foreground">
                          {terminal.networkOperator} • {terminal.signalStrength}%
                        </div>
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={getHealthBadgeVariant(terminal.healthStatus)}
                        className="text-xs"
                      >
                        {terminal.healthStatus}
                      </Badge>
                      {terminal.batteryLevel > 0 && (
                        <div className="text-xs text-muted-foreground mt-1">
                          Battery: {terminal.batteryLevel}%
                        </div>
                      )}
                    </TableCell>
                    <TableCell>
                      <div className="text-xs sm:text-sm">{formatDate(terminal.lastSync)}</div>
                      <div className="text-xs text-muted-foreground">v{terminal.appVersion}</div>
                    </TableCell>
                    <TableCell>
                      <div className="text-xs sm:text-sm font-medium">
                        {terminal.dailyTransactions} txns
                      </div>
                      <div className="text-xs text-muted-foreground">
                        {formatCurrency(terminal.dailyValue)}
                      </div>
                    </TableCell>
                    <TableCell>
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="sm">
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem
                            onClick={() => {
                              setSelectedTerminal(terminal)
                              setShowDetailsModal(true)
                            }}
                          >
                            <Eye className="h-4 w-4 mr-2" />
                            View Details
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            onClick={() => handleTerminalAction('restart', terminal)}
                          >
                            <RotateCcw className="h-4 w-4 mr-2" />
                            Restart
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            onClick={() => handleTerminalAction('update', terminal)}
                          >
                            <Upload className="h-4 w-4 mr-2" />
                            Push Update
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            onClick={() => handleTerminalAction('config', terminal)}
                          >
                            <Settings className="h-4 w-4 mr-2" />
                            Update Config
                          </DropdownMenuItem>
                          <DropdownMenuItem onClick={() => handleTerminalAction('edit', terminal)}>
                            <Edit className="h-4 w-4 mr-2" />
                            Edit Assignment
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            className="text-red-600"
                            onClick={() => handleTerminalAction('decommission', terminal)}
                          >
                            <Trash2 className="h-4 w-4 mr-2" />
                            Decommission
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          {/* Pagination */}
          <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 py-4">
            <div className="text-xs sm:text-sm text-muted-foreground">
              Showing {startIndex + 1} to {Math.min(endIndex, sortedTerminals.length)} of{' '}
              {sortedTerminals.length} terminals
            </div>
            <div className="flex items-center gap-2 w-full sm:w-auto justify-between sm:justify-end">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCurrentPage(prev => Math.max(1, prev - 1))}
                disabled={currentPage <= 1}
                className="text-xs sm:text-sm"
              >
                Previous
              </Button>
              <div className="flex items-center gap-1">
                <span className="text-xs sm:text-sm text-muted-foreground">
                  Page {currentPage} of {totalPages}
                </span>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCurrentPage(prev => Math.min(totalPages, prev + 1))}
                disabled={currentPage >= totalPages}
                className="text-xs sm:text-sm"
              >
                Next
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Terminal Details Modal */}
      <Dialog open={showDetailsModal} onOpenChange={setShowDetailsModal}>
        <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="text-base sm:text-lg">
              Terminal Details - {selectedTerminal?.id}
            </DialogTitle>
            <DialogDescription className="text-xs sm:text-sm">
              Comprehensive information and diagnostics for {selectedTerminal?.retailer}
            </DialogDescription>
          </DialogHeader>
          {selectedTerminal && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 sm:gap-6">
              {/* Left Column */}
              <div className="space-y-4">
                <div>
                  <h4 className="font-semibold mb-2">Device Information</h4>
                  <div className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Terminal ID:</span>
                      <span className="font-mono">{selectedTerminal.id}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">IMEI:</span>
                      <span className="font-mono">{selectedTerminal.imei}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Model:</span>
                      <span>{selectedTerminal.model}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Manufacturer:</span>
                      <span>{selectedTerminal.manufacturer}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">OS Version:</span>
                      <span>{selectedTerminal.osVersion}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">App Version:</span>
                      <span>{selectedTerminal.appVersion}</span>
                    </div>
                  </div>
                </div>

                <div>
                  <h4 className="font-semibold mb-2">Assignment</h4>
                  <div className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Retailer:</span>
                      <span>{selectedTerminal.retailer}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Agent:</span>
                      <span>{selectedTerminal.agent}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Location:</span>
                      <span>{selectedTerminal.location}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Assigned Date:</span>
                      <span>{formatDate(selectedTerminal.assignedDate)}</span>
                    </div>
                  </div>
                </div>
              </div>

              {/* Right Column */}
              <div className="space-y-4">
                <div>
                  <h4 className="font-semibold mb-2">Status & Health</h4>
                  <div className="space-y-2 text-sm">
                    <div className="flex justify-between items-center">
                      <span className="text-muted-foreground">Status:</span>
                      <Badge variant={getStatusBadgeVariant(selectedTerminal.status)}>
                        {selectedTerminal.status}
                      </Badge>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-muted-foreground">Health:</span>
                      <Badge variant={getHealthBadgeVariant(selectedTerminal.healthStatus)}>
                        {selectedTerminal.healthStatus}
                      </Badge>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Connection:</span>
                      <span>{selectedTerminal.isOnline ? 'Online' : 'Offline'}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Network:</span>
                      <span>{selectedTerminal.networkOperator}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Signal:</span>
                      <span>{selectedTerminal.signalStrength}%</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Battery:</span>
                      <span>{selectedTerminal.batteryLevel}%</span>
                    </div>
                  </div>
                </div>

                <div>
                  <h4 className="font-semibold mb-2">Performance</h4>
                  <div className="space-y-2 text-sm">
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Total Transactions:</span>
                      <span>{selectedTerminal.totalTransactions.toLocaleString()}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Total Value:</span>
                      <span>{formatCurrency(selectedTerminal.totalValue)}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Daily Transactions:</span>
                      <span>{selectedTerminal.dailyTransactions}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Daily Value:</span>
                      <span>{formatCurrency(selectedTerminal.dailyValue)}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Last Transaction:</span>
                      <span>{formatDate(selectedTerminal.lastTransaction)}</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-muted-foreground">Last Sync:</span>
                      <span>{formatDate(selectedTerminal.lastSync)}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDetailsModal(false)}>
              Close
            </Button>
            <Button
              onClick={() => {
                setShowDetailsModal(false)
                if (selectedTerminal) {
                  handleTerminalAction('restart', selectedTerminal)
                }
              }}
            >
              Restart Terminal
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
