import React, { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Save, TestTube, AlertCircle, CheckCircle, Settings } from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { toast } from '@/hooks/use-toast'
import { formatCurrency } from '@/lib/utils'
import { winnerSelectionService } from '@/services/winnerSelectionService'

const WinnerSelectionConfig: React.FC = () => {
  const queryClient = useQueryClient()
  
  // Form state
  const [config, setConfig] = useState({
    default_selection_method: 'google_rng' as 'google_rng' | 'cryptographic_rng',
    max_winners_per_game: 1,
    big_win_threshold: 10000, // in pesewas (GHS 100)
    auto_payout_enabled: true,
    audit_retention_days: 365,
    email_notifications: {
      pre_draw: true,
      post_draw: true,
      big_wins: true,
      recipients: [] as string[],
    }
  })
  
  const [newRecipient, setNewRecipient] = useState('')
  const [testConnectionResult, setTestConnectionResult] = useState<any>(null)

  // Fetch current configuration
  const { data: currentConfig, isLoading } = useQuery({
    queryKey: ['winner-selection-config'],
    queryFn: () => winnerSelectionService.getWinnerSelectionConfig(),
  })

  // Sync fetched config into form state
  React.useEffect(() => {
    if (currentConfig) {
      setConfig(currentConfig)
    }
  }, [currentConfig])

  // Test Google RNG connection
  const testConnectionMutation = useMutation({
    mutationFn: () => winnerSelectionService.testGoogleRNGConnection(),
    onSuccess: (result) => {
      setTestConnectionResult(result)
      toast({
        title: result.connected ? 'Connection Successful' : 'Connection Failed',
        description: result.connected 
          ? `API key valid. Rate limit: ${result.rate_limit_remaining} requests remaining`
          : 'Unable to connect to Google RNG service',
        variant: result.connected ? 'default' : 'destructive'
      })
    }
  })

  // Save configuration
  const saveConfigMutation = useMutation({
    mutationFn: (configData: typeof config) => 
      winnerSelectionService.updateWinnerSelectionConfig(configData),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['winner-selection-config'] })
      toast({
        title: 'Configuration Saved',
        description: 'Winner selection configuration has been updated successfully'
      })
    },
    onError: (error: Error) => {
      toast({
        title: 'Save Failed',
        description: error.message,
        variant: 'destructive'
      })
    }
  })

  const handleSave = () => {
    saveConfigMutation.mutate(config)
  }

  const addEmailRecipient = () => {
    if (newRecipient && !config.email_notifications.recipients.includes(newRecipient)) {
      setConfig(prev => ({
        ...prev,
        email_notifications: {
          ...prev.email_notifications,
          recipients: [...prev.email_notifications.recipients, newRecipient]
        }
      }))
      setNewRecipient('')
    }
  }

  const removeEmailRecipient = (email: string) => {
    setConfig(prev => ({
      ...prev,
      email_notifications: {
        ...prev.email_notifications,
        recipients: prev.email_notifications.recipients.filter(r => r !== email)
      }
    }))
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Winner Selection Configuration</h1>
          <p className="text-muted-foreground">
            Configure the winner selection engine and payout settings
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            onClick={() => testConnectionMutation.mutate()}
            disabled={testConnectionMutation.isPending}
          >
            <TestTube className="h-4 w-4 mr-2" />
            Test Google RNG
          </Button>
          <Button
            onClick={handleSave}
            disabled={saveConfigMutation.isPending}
          >
            <Save className="h-4 w-4 mr-2" />
            Save Configuration
          </Button>
        </div>
      </div>

      {/* Connection Status */}
      {testConnectionResult && (
        <Alert variant={testConnectionResult.connected ? 'default' : 'destructive'}>
          {testConnectionResult.connected ? (
            <CheckCircle className="h-4 w-4" />
          ) : (
            <AlertCircle className="h-4 w-4" />
          )}
          <AlertTitle>
            Google RNG Connection {testConnectionResult.connected ? 'Successful' : 'Failed'}
          </AlertTitle>
          <AlertDescription>
            {testConnectionResult.connected ? (
              <>
                API key is valid. Rate limit: {testConnectionResult.rate_limit_remaining} requests remaining.
                {testConnectionResult.test_request_id && (
                  <> Test request ID: {testConnectionResult.test_request_id}</>
                )}
              </>
            ) : (
              'Unable to connect to Google Random Number Generator service. Please check your API configuration.'
            )}
          </AlertDescription>
        </Alert>
      )}

      <div className="grid gap-6 md:grid-cols-2">
        {/* Winner Selection Settings */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Settings className="h-5 w-5" />
              Winner Selection Settings
            </CardTitle>
            <CardDescription>
              Configure how winners are selected for games
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <Label>Default Selection Method</Label>
              <Select 
                value={config.default_selection_method} 
                onValueChange={(value: 'google_rng' | 'cryptographic_rng') => 
                  setConfig(prev => ({ ...prev, default_selection_method: value }))
                }
              >
                <SelectTrigger className="mt-1">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="google_rng">Google Random Number Generator</SelectItem>
                  <SelectItem value="cryptographic_rng">Cryptographic RNG</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground mt-1">
                {config.default_selection_method === 'google_rng' 
                  ? 'Uses Google\'s quantum random number generator for maximum transparency and verifiability'
                  : 'Uses cryptographically secure pseudorandom number generation with audit trails'
                }
              </p>
            </div>

            <div>
              <Label>Maximum Winners per Game</Label>
              <Input
                type="number"
                min="1"
                max="100"
                value={config.max_winners_per_game}
                onChange={(e) => setConfig(prev => ({ 
                  ...prev, 
                  max_winners_per_game: parseInt(e.target.value) || 1 
                }))}
                className="mt-1"
              />
              <p className="text-xs text-muted-foreground mt-1">
                Default number of winning tickets to select per game (can be overridden per draw)
              </p>
            </div>

            <div>
              <Label>Audit Log Retention (Days)</Label>
              <Input
                type="number"
                min="30"
                max="2555" // 7 years
                value={config.audit_retention_days}
                onChange={(e) => setConfig(prev => ({ 
                  ...prev, 
                  audit_retention_days: parseInt(e.target.value) || 365 
                }))}
                className="mt-1"
              />
              <p className="text-xs text-muted-foreground mt-1">
                How long to retain audit logs and cryptographic proofs for transparency
              </p>
            </div>
          </CardContent>
        </Card>

        {/* Payout Settings */}
        <Card>
          <CardHeader>
            <CardTitle>Payout Settings</CardTitle>
            <CardDescription>
              Configure automatic payouts and big win thresholds
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <Label>Big Win Threshold</Label>
              <div className="flex items-center gap-2 mt-1">
                <Input
                  type="number"
                  min="1000"
                  step="100"
                  value={config.big_win_threshold / 100} // Convert from pesewas to cedis
                  onChange={(e) => setConfig(prev => ({ 
                    ...prev, 
                    big_win_threshold: (parseFloat(e.target.value) || 100) * 100 // Convert to pesewas
                  }))}
                />
                <span className="text-sm text-muted-foreground">GHS</span>
              </div>
              <p className="text-xs text-muted-foreground mt-1">
                Wins above {formatCurrency(config.big_win_threshold)} require manual approval
              </p>
            </div>

            <div className="flex items-center justify-between">
              <div>
                <Label>Enable Automatic Payouts</Label>
                <p className="text-xs text-muted-foreground">
                  Automatically process payouts for wins below the big win threshold
                </p>
              </div>
              <Switch
                checked={config.auto_payout_enabled}
                onCheckedChange={(checked) => setConfig(prev => ({ 
                  ...prev, 
                  auto_payout_enabled: checked 
                }))}
              />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Email Notifications */}
      <Card>
        <CardHeader>
          <CardTitle>Email Notifications</CardTitle>
          <CardDescription>
            Configure when and to whom email notifications are sent
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="flex items-center justify-between">
              <div>
                <Label>Pre-Draw Notifications</Label>
                <p className="text-xs text-muted-foreground">
                  Send ticket summary before draw execution
                </p>
              </div>
              <Switch
                checked={config.email_notifications.pre_draw}
                onCheckedChange={(checked) => setConfig(prev => ({ 
                  ...prev, 
                  email_notifications: { ...prev.email_notifications, pre_draw: checked }
                }))}
              />
            </div>

            <div className="flex items-center justify-between">
              <div>
                <Label>Post-Draw Notifications</Label>
                <p className="text-xs text-muted-foreground">
                  Send results after draw completion
                </p>
              </div>
              <Switch
                checked={config.email_notifications.post_draw}
                onCheckedChange={(checked) => setConfig(prev => ({ 
                  ...prev, 
                  email_notifications: { ...prev.email_notifications, post_draw: checked }
                }))}
              />
            </div>

            <div className="flex items-center justify-between">
              <div>
                <Label>Big Win Alerts</Label>
                <p className="text-xs text-muted-foreground">
                  Immediate alerts for big wins requiring approval
                </p>
              </div>
              <Switch
                checked={config.email_notifications.big_wins}
                onCheckedChange={(checked) => setConfig(prev => ({ 
                  ...prev, 
                  email_notifications: { ...prev.email_notifications, big_wins: checked }
                }))}
              />
            </div>
          </div>

          <div>
            <Label>Email Recipients</Label>
            <div className="flex gap-2 mt-1">
              <Input
                type="email"
                placeholder="admin@example.com"
                value={newRecipient}
                onChange={(e) => setNewRecipient(e.target.value)}
                onKeyPress={(e) => e.key === 'Enter' && addEmailRecipient()}
              />
              <Button onClick={addEmailRecipient} variant="outline">
                Add
              </Button>
            </div>
            <div className="flex flex-wrap gap-2 mt-2">
              {config.email_notifications.recipients.map((email) => (
                <Badge key={email} variant="secondary" className="cursor-pointer">
                  {email}
                  <button
                    onClick={() => removeEmailRecipient(email)}
                    className="ml-2 text-xs hover:text-red-600"
                  >
                    ×
                  </button>
                </Badge>
              ))}
            </div>
            <p className="text-xs text-muted-foreground mt-1">
              Administrators who will receive draw and winner notifications
            </p>
          </div>
        </CardContent>
      </Card>

      {/* Security & Compliance */}
      <Card>
        <CardHeader>
          <CardTitle>Security & Compliance</CardTitle>
          <CardDescription>
            Transparency and audit requirements for winner selection
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <Alert>
              <AlertCircle className="h-4 w-4" />
              <AlertTitle>Cryptographic Verification</AlertTitle>
              <AlertDescription>
                All winner selections are cryptographically signed and include:
                <ul className="list-disc list-inside mt-2 space-y-1">
                  <li>Timestamp and request ID for Google RNG calls</li>
                  <li>SHA-256 hash of all ticket entries before selection</li>
                  <li>Digital signature of the selection process</li>
                  <li>Complete audit trail with IP addresses and user actions</li>
                </ul>
              </AlertDescription>
            </Alert>

            <Alert>
              <CheckCircle className="h-4 w-4" />
              <AlertTitle>Compliance Features</AlertTitle>
              <AlertDescription>
                The system ensures regulatory compliance through:
                <ul className="list-disc list-inside mt-2 space-y-1">
                  <li>Pre-draw email notifications with complete ticket manifests</li>
                  <li>Immutable audit logs stored for {config.audit_retention_days} days</li>
                  <li>Cryptographically secure randomization with external verification</li>
                  <li>Manual approval workflow for big wins above {formatCurrency(config.big_win_threshold)}</li>
                </ul>
              </AlertDescription>
            </Alert>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

export default WinnerSelectionConfig