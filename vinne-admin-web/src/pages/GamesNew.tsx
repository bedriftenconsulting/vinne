import { useState } from 'react'
import AdminLayout from '@/components/layouts/AdminLayout'
import { GameList } from '@/components/games/GameList'
import { CreateGameWizard } from '@/components/games/CreateGameWizard'
import { EditGameWizard } from '@/components/games/EditGameWizard'
import { GameDetails } from '@/components/games/GameDetails'
import { PrizeStructureEditor } from '@/components/games/PrizeStructureEditor'
import { ScheduledGamesView } from '@/components/games/ScheduledGamesView'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { type Game, gameService } from '@/services/games'
import { CalendarDays, List, Gamepad2, Play, FileEdit, CalendarCheck, Upload, MessageSquare, AlertCircle, RefreshCw, CheckCircle, XCircle } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import api from '@/lib/api'

// ── Bulk Upload Tab ───────────────────────────────────────────────────────────

function BulkUploadTab() {
  const [selectedDrawId, setSelectedDrawId] = useState('')
  const [bulkRawText, setBulkRawText] = useState('')
  const [bulkParsed, setBulkParsed] = useState<{ phone: string; name: string; quantity: number }[]>([])
  const [bulkParseError, setBulkParseError] = useState('')
  const [bulkUploading, setBulkUploading] = useState(false)
  const [bulkResult, setBulkResult] = useState<{
    total_entries: number
    tickets_created: number
    sms_sent: number
    results: { phone: string; name: string; quantity: number; tickets: string[]; sms_sent: boolean; error?: string }[]
  } | null>(null)

  // Fetch all draws (scheduled + in_progress) to pick from
  const { data: drawsData } = useQuery({
    queryKey: ['all-draws-for-upload'],
    queryFn: async () => {
      const res = await api.get('/admin/draws', { params: { limit: 50 } })
      return res.data?.data?.draws || res.data?.data || []
    },
  })

  const draws: Record<string, any>[] = (drawsData || []).filter((d: any) =>
    !['DRAW_STATUS_COMPLETED', 'DRAW_STATUS_CANCELLED', 'DRAW_STATUS_FAILED'].includes(d.status)
  )

  const selectedDraw = draws.find((d: any) => d.id === selectedDrawId)

  const handlePreview = () => {
    const lines = bulkRawText.split('\n').map(l => l.trim()).filter(Boolean)
    if (lines.length === 0) { setBulkParseError('Paste at least one phone number'); return }
    const parsed = lines.map((line, i) => {
      const parts = line.split(',').map(p => p.trim())
      const phone = parts[0]
      if (!phone) { setBulkParseError(`Line ${i + 1}: phone number is missing`); return null }
      return { phone, name: parts[1] || '', quantity: parseInt(parts[2] || '1', 10) || 1 }
    }).filter(Boolean) as { phone: string; name: string; quantity: number }[]
    setBulkParsed(parsed)
    setBulkParseError('')
  }

  const handleUpload = async () => {
    if (!selectedDrawId) { setBulkParseError('Please select a draw first'); return }
    setBulkUploading(true)
    try {
      const apiBase = import.meta.env.VITE_API_URL || '/api/v1'
      const token = localStorage.getItem('access_token')
      const res = await fetch(`${apiBase}/admin/draws/${selectedDrawId}/tickets/bulk-upload`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ entries: bulkParsed }),
      })
      const text = await res.text()
      let data: Record<string, unknown>
      try { data = JSON.parse(text) } catch {
        setBulkParseError(`Server error (HTTP ${res.status}): ${text.slice(0, 200)}`); return
      }
      if (!res.ok) { setBulkParseError((data?.message as string) || `Upload failed with status ${res.status}`); return }
      setBulkResult((data?.data ?? data) as typeof bulkResult)
    } catch (err) {
      setBulkParseError('Network error: ' + String(err))
    } finally {
      setBulkUploading(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Upload className="h-5 w-5" />
          Bulk Ticket Upload
        </CardTitle>
        <CardDescription>
          Upload tickets for any active game session. Select the draw, paste phone numbers, preview and send.
          <br />
          <span className="font-mono text-xs">Format: phone, name, quantity — e.g. <strong>0241234567, Kwame Mensah, 2</strong></span>
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">

        {/* Draw selector */}
        <div className="space-y-1.5">
          <Label>Select Active Draw *</Label>
          <Select value={selectedDrawId} onValueChange={setSelectedDrawId}>
            <SelectTrigger className="max-w-md">
              <SelectValue placeholder="Choose a draw to upload tickets for..." />
            </SelectTrigger>
            <SelectContent>
              {draws.length === 0 && <SelectItem value="_none" disabled>No active draws found</SelectItem>}
              {draws.map((d: any) => (
                <SelectItem key={d.id} value={d.id}>
                  {d.game_name || d.draw_name} — Draw #{d.draw_number}
                  <span className="ml-2 text-xs text-muted-foreground">
                    ({d.status?.replace('DRAW_STATUS_', '').toLowerCase()})
                  </span>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {selectedDraw && (
            <p className="text-xs text-muted-foreground">
              Draw date: {new Date(selectedDraw.scheduled_time).toLocaleString()} · Schedule ID: {selectedDraw.game_schedule_id?.slice(0, 8)}...
            </p>
          )}
        </div>

        {!bulkResult ? (
          <>
            <div className="space-y-1.5">
              <Label>Phone Numbers</Label>
              <Textarea
                placeholder={`0241234567, Kwame Mensah, 2\n0279876543, Ama Owusu\n0501112233`}
                className="font-mono text-sm min-h-[180px]"
                value={bulkRawText}
                onChange={e => { setBulkRawText(e.target.value); setBulkParsed([]); setBulkParseError('') }}
              />
            </div>

            {bulkParseError && (
              <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertDescription>{bulkParseError}</AlertDescription>
              </Alert>
            )}

            <div className="flex gap-2">
              <Button variant="outline" onClick={handlePreview} disabled={!bulkRawText.trim()}>
                Preview ({bulkRawText.split('\n').filter(l => l.trim()).length} lines)
              </Button>
              {bulkParsed.length > 0 && (
                <Button
                  disabled={bulkUploading || !selectedDrawId}
                  onClick={handleUpload}
                >
                  {bulkUploading
                    ? <><RefreshCw className="h-4 w-4 mr-2 animate-spin" /> Uploading…</>
                    : <><MessageSquare className="h-4 w-4 mr-2" /> Create Tickets & Send SMS ({bulkParsed.length})</>}
                </Button>
              )}
            </div>

            {bulkParsed.length > 0 && (
              <div className="rounded-md border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>#</TableHead>
                      <TableHead>Phone</TableHead>
                      <TableHead>Name</TableHead>
                      <TableHead className="text-center">Tickets</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {bulkParsed.map((entry, i) => (
                      <TableRow key={i}>
                        <TableCell className="text-muted-foreground text-xs">{i + 1}</TableCell>
                        <TableCell className="font-mono text-sm">{entry.phone}</TableCell>
                        <TableCell className="text-sm">{entry.name || <span className="text-muted-foreground italic">—</span>}</TableCell>
                        <TableCell className="text-center"><Badge variant="secondary">{entry.quantity}</Badge></TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
                <div className="p-3 border-t bg-muted/30 text-sm text-muted-foreground">
                  <strong>{bulkParsed.reduce((s, e) => s + e.quantity, 0)}</strong> total tickets across <strong>{bulkParsed.length}</strong> recipients
                </div>
              </div>
            )}
          </>
        ) : (
          <div className="space-y-4">
            <div className="grid grid-cols-3 gap-3">
              <div className="rounded-lg border bg-card p-4 text-center">
                <p className="text-2xl font-bold text-green-500">{bulkResult.tickets_created}</p>
                <p className="text-xs text-muted-foreground mt-1">Tickets Created</p>
              </div>
              <div className="rounded-lg border bg-card p-4 text-center">
                <p className="text-2xl font-bold text-blue-500">{bulkResult.sms_sent}</p>
                <p className="text-xs text-muted-foreground mt-1">SMS Sent</p>
              </div>
              <div className="rounded-lg border bg-card p-4 text-center">
                <p className="text-2xl font-bold">{bulkResult.total_entries}</p>
                <p className="text-xs text-muted-foreground mt-1">Total Recipients</p>
              </div>
            </div>
            <div className="rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Phone</TableHead>
                    <TableHead>Name</TableHead>
                    <TableHead>Tickets</TableHead>
                    <TableHead className="text-center">SMS</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {bulkResult.results?.map((r, i) => (
                    <TableRow key={i}>
                      <TableCell className="font-mono text-sm">{r.phone}</TableCell>
                      <TableCell className="text-sm">{r.name || '—'}</TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {r.tickets?.map((t, j) => <Badge key={j} variant="outline" className="font-mono text-xs">{t}</Badge>)}
                          {(!r.tickets || r.tickets.length === 0) && <span className="text-muted-foreground text-xs">—</span>}
                        </div>
                      </TableCell>
                      <TableCell className="text-center">
                        {r.sms_sent ? <CheckCircle className="h-4 w-4 text-green-500 mx-auto" /> : <XCircle className="h-4 w-4 text-muted-foreground mx-auto" />}
                      </TableCell>
                      <TableCell>
                        {r.error ? <span className="text-destructive text-xs">{r.error}</span> : <span className="text-green-600 text-xs">OK</span>}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <Button variant="outline" onClick={() => { setBulkResult(null); setBulkRawText(''); setBulkParsed([]) }}>
              Upload Another Batch
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ── Stat Cards ────────────────────────────────────────────────────────────────

function GameStatsCards() {
  const { data } = useQuery({
    queryKey: ['games-list'],   // same key as GameList so one fetch serves both
    queryFn: () => gameService.getGames(1, 1000),
    staleTime: 0,
    refetchOnWindowFocus: true,
  })

  const games: Game[] = data?.data || []
  const total   = games.length
  const active  = games.filter(g => g.status?.toLowerCase() === 'active').length
  const draft   = games.filter(g => g.status?.toLowerCase() === 'draft').length
  const running = active // competitions that are live right now

  const cards = [
    { label: 'TOTAL GAMES',   value: total,   sub: 'All games',            icon: Gamepad2,     color: 'text-violet-400' },
    { label: 'ACTIVE GAMES',  value: active,  sub: 'Live & running',       icon: Play,         color: 'text-green-400'  },
    { label: 'DRAFT GAMES',   value: draft,   sub: 'Pending activation',   icon: FileEdit,     color: 'text-blue-400'   },
    { label: 'ACTIVE NOW',    value: running, sub: 'Running competitions',  icon: CalendarCheck,color: 'text-violet-400' },
  ]

  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
      {cards.map(({ label, value, sub, icon: Icon, color }) => (
        <div key={label} className="bg-card border border-border rounded-xl p-5">
          <div className="flex items-center justify-between mb-3">
            <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{label}</span>
            <div className={`w-8 h-8 rounded-full bg-muted flex items-center justify-center ${color}`}>
              <Icon className="h-4 w-4" />
            </div>
          </div>
          <p className="text-3xl font-bold text-foreground">{value}</p>
          <p className="text-xs text-muted-foreground mt-1">{sub}</p>
        </div>
      ))}
    </div>
  )
}

export default function GamesNew() {
  const [activeTab, setActiveTab] = useState('games')
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [selectedGame, setSelectedGame] = useState<Game | null>(null)
  const [isDetailsOpen, setIsDetailsOpen] = useState(false)
  const [isPrizeEditorOpen, setIsPrizeEditorOpen] = useState(false)

  const handleCreateGame = () => setIsCreateDialogOpen(true)

  const handleEditGame = (game: Game) => {
    setSelectedGame(game)
    setIsEditDialogOpen(true)
  }

  const handleViewDetails = (game: Game) => {
    setSelectedGame(game); setIsDetailsOpen(true)
  }

  const handleManageSchedule = (_game: Game) => {
    setActiveTab('scheduler')
  }

  const handleManagePrizes = (game: Game) => {
    setSelectedGame(game); setIsPrizeEditorOpen(true)
  }

  return (
    <AdminLayout>
      <div className="container mx-auto py-6">
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-3xl font-bold">Game Management</h1>
            <p className="text-muted-foreground">Manage competitions and configurations</p>
          </div>
          <button
            onClick={handleCreateGame}
            className="flex items-center gap-2 bg-foreground text-background px-4 py-2 rounded-lg text-sm font-semibold hover:opacity-90 transition"
          >
            + Create Game
          </button>
        </div>

        <GameStatsCards />

        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="w-auto">
            <TabsTrigger value="scheduler" className="gap-2">
              <CalendarDays className="h-4 w-4" />
              Competition Schedule
            </TabsTrigger>
            <TabsTrigger value="games" className="gap-2">
              <List className="h-4 w-4" />
              Competitions List
            </TabsTrigger>
            <TabsTrigger value="bulk-upload" className="gap-2">
              <Upload className="h-4 w-4" />
              Bulk Upload Tickets
            </TabsTrigger>
          </TabsList>

          <TabsContent value="scheduler" className="mt-6">
            <ScheduledGamesView />
          </TabsContent>

          <TabsContent value="games" className="mt-6">
            <GameList
              onCreateGame={handleCreateGame}
              onEditGame={handleEditGame}
              onViewDetails={handleViewDetails}
              onManageSchedule={handleManageSchedule}
              onManagePrizes={handleManagePrizes}
            />
          </TabsContent>

          <TabsContent value="bulk-upload" className="mt-6">
            <BulkUploadTab />
          </TabsContent>
        </Tabs>

        {/* Dialogs */}
        <CreateGameWizard
          isOpen={isCreateDialogOpen}
          onClose={() => { setIsCreateDialogOpen(false); setSelectedGame(null) }}
        />

        {selectedGame && (
          <>
            <EditGameWizard
              game={selectedGame}
              isOpen={isEditDialogOpen}
              onClose={() => { setIsEditDialogOpen(false); setSelectedGame(null) }}
            />
            <GameDetails
              game={selectedGame}
              isOpen={isDetailsOpen}
              onClose={() => { setIsDetailsOpen(false); setSelectedGame(null) }}
              onEdit={() => { setIsDetailsOpen(false); handleEditGame(selectedGame) }}
              onManageSchedule={() => { setIsDetailsOpen(false); handleManageSchedule(selectedGame) }}
              onManagePrizes={() => { setIsDetailsOpen(false); handleManagePrizes(selectedGame) }}
            />
            <PrizeStructureEditor
              game={selectedGame}
              isOpen={isPrizeEditorOpen}
              onClose={() => { setIsPrizeEditorOpen(false); setSelectedGame(null) }}
            />
          </>
        )}
      </div>
    </AdminLayout>
  )
}
