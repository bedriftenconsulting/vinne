import { useState } from 'react'
import AdminLayout from '@/components/layouts/AdminLayout'
import { GameList } from '@/components/games/GameList'
import { CreateGameWizard } from '@/components/games/CreateGameWizard'
import { GameDetails } from '@/components/games/GameDetails'
import { PrizeStructureEditor } from '@/components/games/PrizeStructureEditor'
import { GameCalendar } from '@/components/games/GameCalendar'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { type Game } from '@/services/games'
import { Gamepad2, Calendar, Trophy, TrendingUp } from 'lucide-react'

export default function GamesNew() {
  const [activeTab, setActiveTab] = useState('games')
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [selectedGame, setSelectedGame] = useState<Game | null>(null)
  const [isDetailsOpen, setIsDetailsOpen] = useState(false)
  const [isPrizeEditorOpen, setIsPrizeEditorOpen] = useState(false)
  const [, setIsEditMode] = useState(false)
  const [calendarGame, setCalendarGame] = useState<Game | null>(null)

  const handleCreateGame = () => {
    setIsCreateDialogOpen(true)
  }

  const handleEditGame = (game: Game) => {
    setSelectedGame(game)
    setIsEditMode(true)
    setIsCreateDialogOpen(true)
  }

  const handleViewDetails = (game: Game) => {
    setSelectedGame(game)
    setIsDetailsOpen(true)
  }

  const handleManageSchedule = (game: Game) => {
    setCalendarGame(game)
    setActiveTab('calendar')
  }

  const handleManagePrizes = (game: Game) => {
    setSelectedGame(game)
    setIsPrizeEditorOpen(true)
  }

  return (
    <AdminLayout>
      <div className="container mx-auto py-6">
        <div className="mb-6">
          <h1 className="text-3xl font-bold">Game Management</h1>
          <p className="text-muted-foreground">
            Create and manage lottery games, configure prize structures, and schedule draws
          </p>
        </div>

        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="grid w-full grid-cols-3 lg:w-[500px]">
            <TabsTrigger value="games" className="gap-2">
              <Gamepad2 className="h-4 w-4" />
              Games
            </TabsTrigger>
            <TabsTrigger value="calendar" className="gap-2">
              <Calendar className="h-4 w-4" />
              Calendar
            </TabsTrigger>
            <TabsTrigger value="analytics" className="gap-2">
              <TrendingUp className="h-4 w-4" />
              Analytics
            </TabsTrigger>
          </TabsList>

          <TabsContent value="games" className="mt-6">
            <GameList
              onCreateGame={handleCreateGame}
              onEditGame={handleEditGame}
              onViewDetails={handleViewDetails}
              onManageSchedule={handleManageSchedule}
              onManagePrizes={handleManagePrizes}
            />
          </TabsContent>

          <TabsContent value="calendar" className="mt-6">
            <GameCalendar game={calendarGame || undefined} viewMode="week" />
          </TabsContent>

          <TabsContent value="analytics" className="mt-6">
            <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Total Games</CardTitle>
                  <Gamepad2 className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">12</div>
                  <p className="text-xs text-muted-foreground">+2 from last month</p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Active Games</CardTitle>
                  <Trophy className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">8</div>
                  <p className="text-xs text-muted-foreground">66.7% of total</p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Total Revenue</CardTitle>
                  <TrendingUp className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">₵1.2M</div>
                  <p className="text-xs text-muted-foreground">+15% from last month</p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Total Payouts</CardTitle>
                  <Calendar className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">₵650K</div>
                  <p className="text-xs text-muted-foreground">54% payout ratio</p>
                </CardContent>
              </Card>
            </div>

            <Card className="mt-6">
              <CardHeader>
                <CardTitle>Game Performance Overview</CardTitle>
                <CardDescription>
                  Detailed analytics and performance metrics for all games
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="flex items-center justify-center py-12 text-muted-foreground">
                  Analytics dashboard coming soon...
                </div>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>

        {/* Dialogs */}
        <CreateGameWizard
          isOpen={isCreateDialogOpen}
          onClose={() => {
            setIsCreateDialogOpen(false)
            setIsEditMode(false)
            setSelectedGame(null)
          }}
        />

        {selectedGame && (
          <>
            <GameDetails
              game={selectedGame}
              isOpen={isDetailsOpen}
              onClose={() => {
                setIsDetailsOpen(false)
                setSelectedGame(null)
              }}
              onEdit={() => {
                setIsDetailsOpen(false)
                handleEditGame(selectedGame)
              }}
              onManageSchedule={() => {
                setIsDetailsOpen(false)
                handleManageSchedule(selectedGame)
              }}
              onManagePrizes={() => {
                setIsDetailsOpen(false)
                handleManagePrizes(selectedGame)
              }}
            />

            <PrizeStructureEditor
              game={selectedGame}
              isOpen={isPrizeEditorOpen}
              onClose={() => {
                setIsPrizeEditorOpen(false)
                setSelectedGame(null)
              }}
            />
          </>
        )}
      </div>
    </AdminLayout>
  )
}