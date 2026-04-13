import { useState, useRef, useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { ColorPicker } from '@/components/ui/color-picker'
import { gameService, type Game } from '@/services/games'
import { useToast } from '@/hooks/use-toast'
import { Upload, X, Loader2, Palette } from 'lucide-react'

interface GameBrandingDialogProps {
  isOpen: boolean
  onClose: () => void
  game: Game | null
}

export function GameBrandingDialog({ isOpen, onClose, game }: GameBrandingDialogProps) {
  const [logoFile, setLogoFile] = useState<File | null>(null)
  const [logoPreview, setLogoPreview] = useState<string | null>(null)
  const [brandColor, setBrandColor] = useState<string>('')
  const [isDragging, setIsDragging] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const queryClient = useQueryClient()
  const { toast } = useToast()

  // Initialize with game data when dialog opens
  useEffect(() => {
    if (game && isOpen) {
      setBrandColor(game.brand_color || '')
      setLogoPreview(game.logo_url || null)
      setLogoFile(null)
    }
  }, [game, isOpen])

  const uploadBrandingMutation = useMutation({
    mutationFn: async () => {
      if (!game) throw new Error('No game selected')

      if (logoFile) {
        // Upload logo with brand color
        return await gameService.uploadGameLogo(game.id, logoFile, brandColor || undefined)
      } else if (brandColor && brandColor !== game.brand_color) {
        // Update only brand color using dedicated endpoint
        return await gameService.updateBrandColor(game.id, brandColor)
      }

      throw new Error('No changes to save')
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] })
      toast({
        title: 'Success',
        description: 'Game branding updated successfully',
      })
      onClose()
    },
    onError: (error: Error) => {
      toast({
        title: 'Error',
        description: error.message || 'Failed to update game branding',
        variant: 'destructive',
      })
    },
  })

  const handleFileSelect = (file: File) => {
    // Validate file type
    const allowedTypes = ['image/png', 'image/jpeg', 'image/jpg', 'image/webp']
    if (!allowedTypes.includes(file.type)) {
      toast({
        title: 'Invalid file type',
        description: 'Please upload a PNG, JPEG, or WebP image',
        variant: 'destructive',
      })
      return
    }

    // Validate file size (max 5MB)
    if (file.size > 5 * 1024 * 1024) {
      toast({
        title: 'File too large',
        description: 'Image must be less than 5MB',
        variant: 'destructive',
      })
      return
    }

    setLogoFile(file)

    // Generate preview
    const reader = new FileReader()
    reader.onloadend = () => {
      setLogoPreview(reader.result as string)
    }
    reader.readAsDataURL(file)
  }

  const handleFileInputChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (file) {
      handleFileSelect(file)
    }
  }

  const handleDragOver = (event: React.DragEvent) => {
    event.preventDefault()
    setIsDragging(true)
  }

  const handleDragLeave = () => {
    setIsDragging(false)
  }

  const handleDrop = (event: React.DragEvent) => {
    event.preventDefault()
    setIsDragging(false)

    const file = event.dataTransfer.files[0]
    if (file) {
      handleFileSelect(file)
    }
  }

  const handleRemoveLogo = () => {
    setLogoFile(null)
    setLogoPreview(game?.logo_url || null)
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  const handleSave = () => {
    if (!logoFile && brandColor === (game?.brand_color || '')) {
      toast({
        title: 'No changes',
        description: 'Please make changes before saving',
        variant: 'destructive',
      })
      return
    }

    uploadBrandingMutation.mutate()
  }

  const handleClose = () => {
    if (!uploadBrandingMutation.isPending) {
      setLogoFile(null)
      setLogoPreview(null)
      setBrandColor('')
      onClose()
    }
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Palette className="h-5 w-5" />
            Game Branding
          </DialogTitle>
          <DialogDescription>
            Upload a logo and choose a brand color for {game?.name}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-6 py-4">
          {/* Logo Upload Section */}
          <div className="space-y-3">
            <Label>Game Logo</Label>
            <div
              className={`border-2 border-dashed rounded-lg p-6 transition-colors ${
                isDragging ? 'border-primary bg-primary/5' : 'border-gray-300 hover:border-gray-400'
              }`}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
            >
              {logoPreview ? (
                <div className="space-y-3">
                  <div className="flex items-center justify-center">
                    <img
                      src={logoPreview}
                      alt="Logo preview"
                      className="max-h-32 max-w-full object-contain rounded"
                    />
                  </div>
                  <div className="flex items-center justify-center gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => fileInputRef.current?.click()}
                    >
                      <Upload className="h-4 w-4 mr-2" />
                      Change Logo
                    </Button>
                    {logoFile && (
                      <Button type="button" variant="outline" size="sm" onClick={handleRemoveLogo}>
                        <X className="h-4 w-4 mr-2" />
                        Remove
                      </Button>
                    )}
                  </div>
                </div>
              ) : (
                <div className="text-center space-y-3">
                  <Upload className="h-12 w-12 mx-auto text-gray-400" />
                  <div>
                    <p className="text-sm text-gray-600">
                      Drag and drop your logo here, or{' '}
                      <button
                        type="button"
                        onClick={() => fileInputRef.current?.click()}
                        className="text-primary hover:underline font-medium"
                      >
                        browse
                      </button>
                    </p>
                    <p className="text-xs text-gray-500 mt-1">PNG, JPEG, or WebP (max 5MB)</p>
                  </div>
                </div>
              )}
              <input
                ref={fileInputRef}
                type="file"
                accept="image/png,image/jpeg,image/jpg,image/webp"
                onChange={handleFileInputChange}
                className="hidden"
              />
            </div>
          </div>

          {/* Brand Color Section */}
          <div className="space-y-3">
            <Label>Brand Color</Label>
            <ColorPicker value={brandColor} onChange={setBrandColor} />
            {brandColor && (
              <div className="flex items-center gap-3">
                <div
                  className="h-10 w-10 rounded border border-gray-300"
                  style={{ backgroundColor: brandColor }}
                />
                <span className="text-sm font-mono text-gray-600">{brandColor}</span>
              </div>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={handleClose}
            disabled={uploadBrandingMutation.isPending}
          >
            Cancel
          </Button>
          <Button type="button" onClick={handleSave} disabled={uploadBrandingMutation.isPending}>
            {uploadBrandingMutation.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
            Save Branding
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
