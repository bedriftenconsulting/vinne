import { useEffect, useState } from 'react'
import AdminLayout from '@/components/layouts/AdminLayout'
import { Card, CardContent } from '@/components/ui/card'
// import { Button } from '@/components/ui/button' // Commented out - unused after hiding Generate Report button
import {
  DollarSign,
  // FileText, // Commented out - unused after hiding Generate Report button
  TrendingUp,
  TrendingDown,
  Ticket,
  Coins,
  CheckCircle,
  XCircle,
  AlertCircle,
  Percent,
  Users,
} from 'lucide-react'
// Charts imports commented out temporarily
// import {
//   LineChart,
//   Line,
//   AreaChart,
//   Area,
//   BarChart,
//   Bar,
//   XAxis,
//   YAxis,
//   CartesianGrid,
//   Tooltip,
//   Legend,
//   ResponsiveContainer,
// } from 'recharts'

interface DashboardMetrics {
  totalGrossRevenue: number
  totalTickets: number
  totalPayouts: number
  winRate: number
  monthlyRevenueChange: number
  monthlyTicketsChange: number
  monthlyPayoutsChange: number
  // New metrics
  totalStakes: number
  totalStakesAmount: number
  totalPaidTickets: number
  totalPaymentsAmount: number
  totalUnpaidTickets: number
  totalUnpaidAmount: number
  totalCommissions: number
  totalRetailers: number
  stakesChange: number
  stakesAmountChange: number
  paidTicketsChange: number
  paymentsAmountChange: number
  unpaidTicketsChange: number
  unpaidAmountChange: number
  commissionsChange: number
}

// Temporarily disabled for charts and top agents sections
// interface ChartData {
//   month: string
//   revenue: number
//   tickets: number
//   payouts: number
// }

// interface TopAgent {
//   id: string
//   agent_code: string
//   name: string
//   revenue: number
//   tickets: number
//   retailer_count: number
// }

export default function Dashboard() {
  const [loading, setLoading] = useState(true)
  const [metrics, setMetrics] = useState<DashboardMetrics>({
    totalGrossRevenue: 0,
    totalTickets: 0,
    totalPayouts: 0,
    winRate: 0,
    monthlyRevenueChange: 0,
    monthlyTicketsChange: 0,
    monthlyPayoutsChange: 0,
    // New metrics
    totalStakes: 0,
    totalStakesAmount: 0,
    totalPaidTickets: 0,
    totalPaymentsAmount: 0,
    totalUnpaidTickets: 0,
    totalUnpaidAmount: 0,
    totalCommissions: 0,
    totalRetailers: 0,
    stakesChange: 0,
    stakesAmountChange: 0,
    paidTicketsChange: 0,
    paymentsAmountChange: 0,
    unpaidTicketsChange: 0,
    unpaidAmountChange: 0,
    commissionsChange: 0,
  })

  // Temporarily disabled for charts and top agents sections
  // const [chartData, setChartData] = useState<ChartData[]>([])
  // const [topAgents, setTopAgents] = useState<TopAgent[]>([])

  useEffect(() => {
    const fetchData = async () => {
      try {
        // Fetch real daily metrics from API
        const { dashboardService } = await import('@/services/dashboard')
        const dailyMetrics = await dashboardService.getDailyMetrics()

        setMetrics({
          totalGrossRevenue: dailyMetrics.metrics.gross_revenue.amount_ghs,
          totalTickets: dailyMetrics.metrics.tickets.count,
          totalPayouts: dailyMetrics.metrics.payouts.amount_ghs,
          winRate: dailyMetrics.metrics.win_rate.percentage,
          monthlyRevenueChange: dailyMetrics.metrics.gross_revenue.change_percentage,
          monthlyTicketsChange: dailyMetrics.metrics.tickets.change_percentage,
          monthlyPayoutsChange: dailyMetrics.metrics.payouts.change_percentage,
          // New metrics
          totalStakes: dailyMetrics.metrics.stakes.count,
          totalStakesAmount: dailyMetrics.metrics.stakes_amount.amount_ghs,
          totalPaidTickets: dailyMetrics.metrics.paid_tickets.count,
          totalPaymentsAmount: dailyMetrics.metrics.payments_amount.amount_ghs,
          totalUnpaidTickets: dailyMetrics.metrics.unpaid_tickets.count,
          totalUnpaidAmount: dailyMetrics.metrics.unpaid_amount.amount_ghs,
          totalCommissions: dailyMetrics.metrics.commissions?.amount_ghs || 0,
          totalRetailers: dailyMetrics.metrics.retailers?.count || 0,
          stakesChange: dailyMetrics.metrics.stakes.change_percentage,
          stakesAmountChange: dailyMetrics.metrics.stakes_amount.change_percentage,
          paidTicketsChange: dailyMetrics.metrics.paid_tickets.change_percentage,
          paymentsAmountChange: dailyMetrics.metrics.payments_amount.change_percentage,
          unpaidTicketsChange: dailyMetrics.metrics.unpaid_tickets.change_percentage,
          unpaidAmountChange: dailyMetrics.metrics.unpaid_amount.change_percentage,
          commissionsChange: dailyMetrics.metrics.commissions?.change_percentage || 0,
        })

        // Temporarily disabled - Fetch monthly metrics for charts
        // const monthlyMetrics = await dashboardService.getMonthlyMetrics(6)

        // // Convert month format from YYYY-MM to short month name (Jan, Feb, etc.)
        // const monthNames = [
        //   'Jan',
        //   'Feb',
        //   'Mar',
        //   'Apr',
        //   'May',
        //   'Jun',
        //   'Jul',
        //   'Aug',
        //   'Sep',
        //   'Oct',
        //   'Nov',
        //   'Dec',
        // ]
        // const formattedChartData = monthlyMetrics.data.map(dataPoint => {
        //   const [, monthNum] = dataPoint.month.split('-')
        //   const monthIndex = parseInt(monthNum, 10) - 1
        //   return {
        //     month: monthNames[monthIndex],
        //     revenue: dataPoint.revenue_ghs,
        //     tickets: dataPoint.tickets,
        //     payouts: dataPoint.payouts_ghs,
        //   }
        // })

        // setChartData(formattedChartData)

        // // Fetch top performing agents
        // const topAgentsData = await dashboardService.getTopPerformingAgents({
        //   period: 'monthly',
        //   limit: 10,
        // })

        // setTopAgents(topAgentsData.agents)
      } catch (error) {
        console.error('Failed to fetch data:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [])

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('en-GH', {
      style: 'currency',
      currency: 'GHS',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount)
  }

  const formatNumber = (num: number) => {
    return new Intl.NumberFormat('en-GH').format(num)
  }

  if (loading) {
    return (
      <AdminLayout>
        <div className="flex items-center justify-center h-64">
          <div className="flex items-center space-x-2">
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-emerald-600"></div>
            <span className="text-gray-600">Loading dashboard...</span>
          </div>
        </div>
      </AdminLayout>
    )
  }

  // Temporarily disabled - Custom tooltip for charts
  // const CustomTooltip = ({
  //   active,
  //   payload,
  //   label,
  // }: {
  //   active?: boolean
  //   payload?: Array<{ color: string; name: string; value: number }>
  //   label?: string
  // }) => {
  //   if (active && payload && payload.length) {
  //     return (
  //       <div className="bg-white p-3 border border-gray-200 rounded-lg shadow-lg">
  //         <p className="text-sm font-semibold mb-1">{label}</p>
  //         {payload.map((entry: { color: string; name: string; value: number }, index: number) => (
  //           <p key={index} className="text-xs" style={{ color: entry.color }}>
  //             {entry.name}:{' '}
  //             {entry.name === 'Tickets' ? formatNumber(entry.value) : formatCurrency(entry.value)}
  //           </p>
  //         ))}
  //       </div>
  //     )
  //   }
  //   return null
  // }

  return (
    <AdminLayout>
      <div className="p-3 sm:p-4 md:p-6 max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-4 sm:mb-6 md:mb-8">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
            <div className="min-w-0">
              <h1 className="text-2xl sm:text-3xl font-bold tracking-tight">Dashboard</h1>
              <p className="text-muted-foreground mt-1 text-sm sm:text-base">
                Comprehensive overview of lottery operations
              </p>
            </div>
            {/* TEMPORARILY HIDDEN - Generate Report Button */}
            {/* <div className="flex items-center space-x-3 shrink-0">
              <Button size="sm" className="text-xs sm:text-sm">
                <FileText className="w-3 h-3 sm:w-4 sm:h-4 mr-1 sm:mr-2" />
                <span className="hidden sm:inline">Generate Report</span>
                <span className="sm:hidden">Report</span>
              </Button>
            </div> */}
          </div>
        </div>

        {/* Key Metrics Cards */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 sm:gap-6 md:gap-8 mb-4 sm:mb-6 md:mb-8">
          {/* Total Stakes */}
          <Card>
            <CardContent className="p-6 sm:p-8">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1 pr-3">
                  <p className="text-sm sm:text-base font-medium text-teal-600">Total Stakes</p>
                  <p className="text-2xl sm:text-3xl md:text-4xl font-bold text-teal-900 truncate">
                    {formatNumber(metrics.totalStakes)}
                  </p>
                  <div className="flex items-center mt-2">
                    {metrics.stakesChange > 0 ? (
                      <TrendingUp className="w-4 h-4 text-green-600 mr-1 shrink-0" />
                    ) : (
                      <TrendingDown className="w-4 h-4 text-red-600 mr-1 shrink-0" />
                    )}
                    <p
                      className={`text-sm ${metrics.stakesChange > 0 ? 'text-green-700' : 'text-red-700'}`}
                    >
                      {Math.abs(metrics.stakesChange)}%{' '}
                      <span className="hidden sm:inline">(Daily)</span>
                    </p>
                  </div>
                </div>
                <div className="w-12 h-12 sm:w-16 sm:h-16 bg-teal-200 rounded-lg flex items-center justify-center shrink-0">
                  <Ticket className="w-6 h-6 sm:w-8 sm:h-8 text-teal-600" />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Total Stakes Amount */}
          <Card>
            <CardContent className="p-6 sm:p-8">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1 pr-3">
                  <p className="text-sm sm:text-base font-medium text-indigo-600">Stakes Amount</p>
                  <p className="text-2xl sm:text-3xl md:text-4xl font-bold text-indigo-900 truncate">
                    {formatCurrency(metrics.totalStakesAmount)}
                  </p>
                  <div className="flex items-center mt-2">
                    {metrics.stakesAmountChange > 0 ? (
                      <TrendingUp className="w-4 h-4 text-green-600 mr-1 shrink-0" />
                    ) : (
                      <TrendingDown className="w-4 h-4 text-red-600 mr-1 shrink-0" />
                    )}
                    <p
                      className={`text-sm ${metrics.stakesAmountChange > 0 ? 'text-green-700' : 'text-red-700'}`}
                    >
                      {Math.abs(metrics.stakesAmountChange)}%{' '}
                      <span className="hidden sm:inline">(Daily)</span>
                    </p>
                  </div>
                </div>
                <div className="w-12 h-12 sm:w-16 sm:h-16 bg-indigo-200 rounded-lg flex items-center justify-center shrink-0">
                  <Coins className="w-6 h-6 sm:w-8 sm:h-8 text-indigo-600" />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Total Payments (Paid Tickets) */}
          <Card>
            <CardContent className="p-6 sm:p-8">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1 pr-3">
                  <p className="text-sm sm:text-base font-medium text-emerald-600">
                    Total Payments
                  </p>
                  <p className="text-2xl sm:text-3xl md:text-4xl font-bold text-emerald-900 truncate">
                    {formatNumber(metrics.totalPaidTickets)}
                  </p>
                  <div className="flex items-center mt-2">
                    {metrics.paidTicketsChange > 0 ? (
                      <TrendingUp className="w-4 h-4 text-green-600 mr-1 shrink-0" />
                    ) : (
                      <TrendingDown className="w-4 h-4 text-red-600 mr-1 shrink-0" />
                    )}
                    <p
                      className={`text-sm ${metrics.paidTicketsChange > 0 ? 'text-green-700' : 'text-red-700'}`}
                    >
                      {Math.abs(metrics.paidTicketsChange)}%{' '}
                      <span className="hidden sm:inline">(Daily)</span>
                    </p>
                  </div>
                </div>
                <div className="w-12 h-12 sm:w-16 sm:h-16 bg-emerald-200 rounded-lg flex items-center justify-center shrink-0">
                  <CheckCircle className="w-6 h-6 sm:w-8 sm:h-8 text-emerald-600" />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Total Payments Amount */}
          <Card>
            <CardContent className="p-6 sm:p-8">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1 pr-3">
                  <p className="text-sm sm:text-base font-medium text-cyan-600">Payments Amount</p>
                  <p className="text-2xl sm:text-3xl md:text-4xl font-bold text-cyan-900 truncate">
                    {formatCurrency(metrics.totalPaymentsAmount)}
                  </p>
                  <div className="flex items-center mt-2">
                    {metrics.paymentsAmountChange > 0 ? (
                      <TrendingUp className="w-4 h-4 text-green-600 mr-1 shrink-0" />
                    ) : (
                      <TrendingDown className="w-4 h-4 text-red-600 mr-1 shrink-0" />
                    )}
                    <p
                      className={`text-sm ${metrics.paymentsAmountChange > 0 ? 'text-green-700' : 'text-red-700'}`}
                    >
                      {Math.abs(metrics.paymentsAmountChange)}%{' '}
                      <span className="hidden sm:inline">(Daily)</span>
                    </p>
                  </div>
                </div>
                <div className="w-12 h-12 sm:w-16 sm:h-16 bg-cyan-200 rounded-lg flex items-center justify-center shrink-0">
                  <DollarSign className="w-6 h-6 sm:w-8 sm:h-8 text-cyan-600" />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Unpaid Tickets */}
          <Card>
            <CardContent className="p-6 sm:p-8">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1 pr-3">
                  <p className="text-sm sm:text-base font-medium text-amber-600">Unpaid Tickets</p>
                  <p className="text-2xl sm:text-3xl md:text-4xl font-bold text-amber-900 truncate">
                    {formatNumber(metrics.totalUnpaidTickets)}
                  </p>
                  <div className="flex items-center mt-2">
                    {metrics.unpaidTicketsChange > 0 ? (
                      <TrendingUp className="w-4 h-4 text-amber-600 mr-1 shrink-0" />
                    ) : (
                      <TrendingDown className="w-4 h-4 text-green-600 mr-1 shrink-0" />
                    )}
                    <p
                      className={`text-sm ${metrics.unpaidTicketsChange > 0 ? 'text-amber-700' : 'text-green-700'}`}
                    >
                      {Math.abs(metrics.unpaidTicketsChange)}%{' '}
                      <span className="hidden sm:inline">(Daily)</span>
                    </p>
                  </div>
                </div>
                <div className="w-12 h-12 sm:w-16 sm:h-16 bg-amber-200 rounded-lg flex items-center justify-center shrink-0">
                  <XCircle className="w-6 h-6 sm:w-8 sm:h-8 text-amber-600" />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Unpaid Tickets Amount */}
          <Card>
            <CardContent className="p-6 sm:p-8">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1 pr-3">
                  <p className="text-sm sm:text-base font-medium text-red-600">Unpaid Amount</p>
                  <p className="text-2xl sm:text-3xl md:text-4xl font-bold text-red-900 truncate">
                    {formatCurrency(metrics.totalUnpaidAmount)}
                  </p>
                  <div className="flex items-center mt-2">
                    {metrics.unpaidAmountChange > 0 ? (
                      <TrendingUp className="w-4 h-4 text-red-600 mr-1 shrink-0" />
                    ) : (
                      <TrendingDown className="w-4 h-4 text-green-600 mr-1 shrink-0" />
                    )}
                    <p
                      className={`text-sm ${metrics.unpaidAmountChange > 0 ? 'text-red-700' : 'text-green-700'}`}
                    >
                      {Math.abs(metrics.unpaidAmountChange)}%{' '}
                      <span className="hidden sm:inline">(Daily)</span>
                    </p>
                  </div>
                </div>
                <div className="w-12 h-12 sm:w-16 sm:h-16 bg-red-200 rounded-lg flex items-center justify-center shrink-0">
                  <AlertCircle className="w-6 h-6 sm:w-8 sm:h-8 text-red-600" />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Commissions */}
          <Card>
            <CardContent className="p-6 sm:p-8">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1 pr-3">
                  <p className="text-sm sm:text-base font-medium text-violet-600">Commissions</p>
                  <p className="text-2xl sm:text-3xl md:text-4xl font-bold text-violet-900 truncate">
                    {formatCurrency(metrics.totalCommissions)}
                  </p>
                  <div className="flex items-center mt-2">
                    {metrics.commissionsChange > 0 ? (
                      <TrendingUp className="w-4 h-4 text-green-600 mr-1 shrink-0" />
                    ) : (
                      <TrendingDown className="w-4 h-4 text-red-600 mr-1 shrink-0" />
                    )}
                    <p
                      className={`text-sm ${metrics.commissionsChange > 0 ? 'text-green-700' : 'text-red-700'}`}
                    >
                      {Math.abs(metrics.commissionsChange)}%{' '}
                      <span className="hidden sm:inline">(Daily)</span>
                    </p>
                  </div>
                </div>
                <div className="w-12 h-12 sm:w-16 sm:h-16 bg-violet-200 rounded-lg flex items-center justify-center shrink-0">
                  <Percent className="w-6 h-6 sm:w-8 sm:h-8 text-violet-600" />
                </div>
              </div>
            </CardContent>
          </Card>

          {/* Total Retailers */}
          <Card>
            <CardContent className="p-6 sm:p-8">
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1 pr-3">
                  <p className="text-sm sm:text-base font-medium text-sky-600">Total Retailers</p>
                  <p className="text-2xl sm:text-3xl md:text-4xl font-bold text-sky-900 truncate">
                    {formatNumber(metrics.totalRetailers)}
                  </p>
                  <p className="text-sm text-sky-700">Active retailers</p>
                </div>
                <div className="w-12 h-12 sm:w-16 sm:h-16 bg-sky-200 rounded-lg flex items-center justify-center shrink-0">
                  <Users className="w-6 h-6 sm:w-8 sm:h-8 text-sky-600" />
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Charts Section */}
        {/* TEMPORARILY HIDDEN - Charts Section */}
        {/* <div className="grid grid-cols-1 lg:grid-cols-2 gap-3 sm:gap-4 md:gap-6 mb-4 sm:mb-6 md:mb-8"> */}
        {/* Revenue Chart */}
        {/* <Card>
            <CardHeader className="p-4 sm:p-6">
              <CardTitle className="text-base sm:text-lg">Revenue Trend</CardTitle>
              <CardDescription className="text-xs sm:text-sm">
                Monthly revenue over the last 6 months
              </CardDescription>
            </CardHeader>
            <CardContent className="p-4 sm:p-6 pt-0">
              <ResponsiveContainer width="100%" height={200} className="sm:h-[250px]">
                <AreaChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="month" />
                  <YAxis tickFormatter={value => `${value / 1000}k`} />
                  <Tooltip content={<CustomTooltip />} />
                  <Area
                    type="monotone"
                    dataKey="revenue"
                    stroke="#10b981"
                    fill="#10b981"
                    fillOpacity={0.3}
                    strokeWidth={2}
                    name="Revenue"
                  />
                </AreaChart>
              </ResponsiveContainer>
            </CardContent>
          </Card> */}

        {/* Tickets Chart */}
        {/* <Card>
            <CardHeader className="p-4 sm:p-6">
              <CardTitle className="text-base sm:text-lg">Tickets Sold</CardTitle>
              <CardDescription className="text-xs sm:text-sm">
                Monthly ticket sales over the last 6 months
              </CardDescription>
            </CardHeader>
            <CardContent className="p-4 sm:p-6 pt-0">
              <ResponsiveContainer width="100%" height={200} className="sm:h-[250px]">
                <BarChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="month" />
                  <YAxis tickFormatter={value => `${value / 1000}k`} />
                  <Tooltip content={<CustomTooltip />} />
                  <Bar dataKey="tickets" fill="#3b82f6" radius={[8, 8, 0, 0]} name="Tickets" />
                </BarChart>
              </ResponsiveContainer>
            </CardContent>
          </Card> */}

        {/* Payouts Chart */}
        {/* <Card>
            <CardHeader className="p-4 sm:p-6">
              <CardTitle className="text-base sm:text-lg">Payouts Trend</CardTitle>
              <CardDescription className="text-xs sm:text-sm">
                Monthly payouts over the last 6 months
              </CardDescription>
            </CardHeader>
            <CardContent className="p-4 sm:p-6 pt-0">
              <ResponsiveContainer width="100%" height={200} className="sm:h-[250px]">
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="month" />
                  <YAxis tickFormatter={value => `${value / 1000}k`} />
                  <Tooltip content={<CustomTooltip />} />
                  <Line
                    type="monotone"
                    dataKey="payouts"
                    stroke="#a855f7"
                    strokeWidth={2}
                    dot={{ fill: '#a855f7', r: 4 }}
                    name="Payouts"
                  />
                </LineChart>
              </ResponsiveContainer>
            </CardContent>
          </Card> */}

        {/* Combined Chart - All three metrics */}
        {/* <Card>
            <CardHeader className="p-4 sm:p-6">
              <CardTitle className="text-base sm:text-lg">Combined Overview</CardTitle>
              <CardDescription className="text-xs sm:text-sm">
                Revenue, Tickets, and Payouts comparison
              </CardDescription>
            </CardHeader>
            <CardContent className="p-4 sm:p-6 pt-0">
              <ResponsiveContainer width="100%" height={200} className="sm:h-[250px]">
                <LineChart data={chartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="month" />
                  <YAxis yAxisId="left" tickFormatter={value => `${value / 1000}k`} />
                  <YAxis
                    yAxisId="right"
                    orientation="right"
                    tickFormatter={value => `${value / 1000}k`}
                  />
                  <Tooltip content={<CustomTooltip />} />
                  <Legend />
                  <Line
                    yAxisId="left"
                    type="monotone"
                    dataKey="revenue"
                    stroke="#10b981"
                    strokeWidth={2}
                    name="Revenue"
                  />
                  <Line
                    yAxisId="right"
                    type="monotone"
                    dataKey="tickets"
                    stroke="#3b82f6"
                    strokeWidth={2}
                    name="Tickets"
                  />
                  <Line
                    yAxisId="left"
                    type="monotone"
                    dataKey="payouts"
                    stroke="#a855f7"
                    strokeWidth={2}
                    name="Payouts"
                  />
                </LineChart>
              </ResponsiveContainer>
            </CardContent>
          </Card> */}
        {/* </div> */}

        {/* Top Agents Section */}
        {/* TEMPORARILY HIDDEN - Top Performing Agents Section */}
        {/* <Card>
          <CardHeader className="p-4 sm:p-6">
            <CardTitle className="text-base sm:text-lg">Top Performing Agents</CardTitle>
            <CardDescription className="text-xs sm:text-sm">
              Leading agents by revenue this month
            </CardDescription>
          </CardHeader>
          <CardContent className="p-0 sm:p-6 sm:pt-0">
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-gray-200">
                    <th className="text-left py-2 sm:py-3 px-2 sm:px-4 text-xs sm:text-sm font-medium text-gray-700">
                      Rank
                    </th>
                    <th className="text-left py-2 sm:py-3 px-2 sm:px-4 text-xs sm:text-sm font-medium text-gray-700">
                      Agent Name
                    </th>
                    <th className="text-right py-2 sm:py-3 px-2 sm:px-4 text-xs sm:text-sm font-medium text-gray-700">
                      Revenue
                    </th>
                    <th className="text-right py-2 sm:py-3 px-2 sm:px-4 text-xs sm:text-sm font-medium text-gray-700 hidden sm:table-cell">
                      Tickets
                    </th>
                    <th className="text-right py-2 sm:py-3 px-2 sm:px-4 text-xs sm:text-sm font-medium text-gray-700 hidden md:table-cell">
                      Retailers
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {topAgents.map((agent, index) => (
                    <tr key={agent.id} className="border-b border-gray-100 hover:bg-gray-50">
                      <td className="py-2 sm:py-3 px-2 sm:px-4">
                        <div className="flex items-center">
                          {index === 0 && (
                            <Trophy className="w-3 h-3 sm:w-4 sm:h-4 text-yellow-500 mr-1 sm:mr-2 shrink-0" />
                          )}
                          {index === 1 && (
                            <Trophy className="w-3 h-3 sm:w-4 sm:h-4 text-gray-400 mr-1 sm:mr-2 shrink-0" />
                          )}
                          {index === 2 && (
                            <Trophy className="w-3 h-3 sm:w-4 sm:h-4 text-orange-600 mr-1 sm:mr-2 shrink-0" />
                          )}
                          <span className="font-semibold text-xs sm:text-sm">{index + 1}</span>
                        </div>
                      </td>
                      <td className="py-2 sm:py-3 px-2 sm:px-4 font-medium text-gray-900 text-xs sm:text-sm truncate max-w-32 sm:max-w-none">
                        {agent.name}
                      </td>
                      <td className="py-2 sm:py-3 px-2 sm:px-4 text-right font-semibold text-green-600 text-xs sm:text-sm whitespace-nowrap">
                        {formatCurrency(agent.revenue)}
                      </td>
                      <td className="py-2 sm:py-3 px-2 sm:px-4 text-right text-gray-700 text-xs sm:text-sm hidden sm:table-cell">
                        {formatNumber(agent.tickets)}
                      </td>
                      <td className="py-2 sm:py-3 px-2 sm:px-4 text-right text-gray-700 text-xs sm:text-sm hidden md:table-cell">
                        {agent.retailer_count}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card> */}
      </div>
    </AdminLayout>
  )
}
