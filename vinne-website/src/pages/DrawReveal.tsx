import React, { useState, useEffect, useRef, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import confetti from 'canvas-confetti'
import logoSrc from '@/assets/logo.png'
import prizeSrc from '@/assets/prize-iphone17.jpg'

const BASE = import.meta.env.VITE_API_URL || '/api/v1'

type Stage = 'loading' | 'ready' | 'countdown' | 'rolling' | 'reveal' | 'celebrating'

interface Winner {
  serial_number?: string
  game_name?: string
  customer_phone?: string
  customer_name?: string
  draw_number?: number
}

// Characters to cycle through during slot-machine roll
const SLOT_CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'

function randomChar() {
  return SLOT_CHARS[Math.floor(Math.random() * SLOT_CHARS.length)]
}

function maskPhone(phone?: string) {
  if (!phone) return '***'
  const s = phone.replace(/\D/g, '')
  if (s.length < 6) return '****'
  return s.slice(0, 3) + '****' + s.slice(-3)
}

// Star particle component
function Stars() {
  const stars = Array.from({ length: 60 }, (_, i) => ({
    id: i,
    x: Math.random() * 100,
    y: Math.random() * 100,
    size: Math.random() * 2.5 + 0.5,
    delay: Math.random() * 4,
    duration: Math.random() * 3 + 2,
  }))
  return (
    <div className="absolute inset-0 overflow-hidden pointer-events-none">
      {stars.map(s => (
        <motion.div
          key={s.id}
          className="absolute rounded-full bg-yellow-200"
          style={{ left: `${s.x}%`, top: `${s.y}%`, width: s.size, height: s.size }}
          animate={{ opacity: [0.1, 0.9, 0.1], scale: [1, 1.5, 1] }}
          transition={{ duration: s.duration, delay: s.delay, repeat: Infinity, ease: 'easeInOut' }}
        />
      ))}
    </div>
  )
}

// Single spinning slot character
function SlotChar({ char, locked }: { char: string; locked: boolean }) {
  return (
    <motion.span
      className={`inline-block font-black tabular-nums ${
        locked ? 'text-yellow-300' : 'text-green-400'
      }`}
      animate={locked ? { scale: [1.3, 1], color: '#fde047' } : {}}
      transition={{ duration: 0.3 }}
    >
      {char}
    </motion.span>
  )
}

export default function DrawReveal() {
  const [stage, setStage] = useState<Stage>('loading')
  const [winner, setWinner] = useState<Winner | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [countdownNum, setCountdownNum] = useState(3)
  const [rollingChars, setRollingChars] = useState<string[]>([])
  const [lockedCount, setLockedCount] = useState(0)
  const rollingRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const ticketChars = winner?.serial_number?.split('') ?? []

  // Fetch winner from public endpoint
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const drawId = params.get('drawId')
    const url = drawId
      ? `${BASE}/draws/${drawId}/results`
      : `${BASE}/public/winners?limit=1`

    fetch(url)
      .then(r => r.json())
      .then(d => {
        // /public/winners returns { data: { winners: [...] } }
        // /draws/{id}/results returns { data: { winning_tickets: [...] } }
        const winners = d?.data?.winners ?? d?.data?.winning_tickets ?? d?.data?.tickets ?? []
        const list = Array.isArray(winners) ? winners : []
        if (list.length === 0) {
          setError('No draw results available yet.')
          setStage('ready')
          return
        }
        const raw = list[0]
        // Normalise field names from both endpoints
        const won: Winner = {
          serial_number: raw.serial_number || raw.ticket_serial || raw.ticket_number,
          game_name: raw.game_name || raw.prize || raw.game_code,
          customer_phone: raw.customer_phone || raw.phone_number || raw.phone,
          customer_name: raw.customer_name || raw.player_name || raw.name,
          draw_number: raw.draw_number,
        }
        setWinner(won)
        setStage('ready')
      })
      .catch(() => {
        setError('Could not load draw results')
        setStage('ready')
      })
  }, [])

  const fireConfetti = useCallback(() => {
    const burst = (opts: confetti.Options) => confetti({ ...opts, disableForReducedMotion: true })
    const colors = ['#fde047', '#f59e0b', '#10b981', '#3b82f6', '#ef4444', '#ffffff']
    burst({ particleCount: 120, spread: 80, origin: { y: 0.6 }, colors })
    setTimeout(() => burst({ particleCount: 80, spread: 100, origin: { y: 0.5, x: 0.2 }, colors }), 300)
    setTimeout(() => burst({ particleCount: 80, spread: 100, origin: { y: 0.5, x: 0.8 }, colors }), 500)
    setTimeout(() => burst({ particleCount: 60, spread: 120, origin: { y: 0.3 }, colors }), 800)
  }, [])

  const startReveal = useCallback(() => {
    setStage('countdown')
    setCountdownNum(3)

    let count = 3
    const tick = setInterval(() => {
      count -= 1
      if (count <= 0) {
        clearInterval(tick)
        startRolling()
      } else {
        setCountdownNum(count)
      }
    }, 1000)
  }, [winner]) // eslint-disable-line react-hooks/exhaustive-deps

  const startRolling = useCallback(() => {
    if (!winner?.serial_number) {
      setStage('reveal')
      return
    }
    const chars = winner.serial_number.split('')
    setRollingChars(chars.map(c => (c === '-' ? '-' : randomChar())))
    setLockedCount(0)
    setStage('rolling')

    let locked = 0
    // Roll random chars every 80ms
    rollingRef.current = setInterval(() => {
      setRollingChars(prev =>
        prev.map((_, i) => {
          if (chars[i] === '-') return '-'
          if (i < locked) return chars[i]
          return randomChar()
        })
      )
    }, 80)

    // Lock in one character at a time, starting after 1.5s
    const lockNext = (idx: number) => {
      if (idx >= chars.length) {
        if (rollingRef.current) clearInterval(rollingRef.current)
        setRollingChars(chars)
        setTimeout(() => {
          setStage('reveal')
          setTimeout(() => {
            setStage('celebrating')
            fireConfetti()
          }, 800)
        }, 400)
        return
      }
      locked = idx + 1
      setLockedCount(idx + 1)
      const delay = chars[idx] === '-' ? 80 : 250
      setTimeout(() => lockNext(idx + 1), delay)
    }

    setTimeout(() => lockNext(0), 1500)
  }, [winner, fireConfetti])

  // Cleanup
  useEffect(() => () => { if (rollingRef.current) clearInterval(rollingRef.current) }, [])

  return (
    <div
      className="fixed inset-0 flex flex-col items-center justify-center overflow-hidden select-none"
      style={{ background: 'radial-gradient(ellipse at center, #0f0f1a 0%, #000000 100%)' }}
    >
      <Stars />

      {/* Gold ring glow behind ticket number during reveal */}
      <AnimatePresence>
        {(stage === 'reveal' || stage === 'celebrating') && (
          <motion.div
            className="absolute rounded-full"
            style={{
              width: 400, height: 400,
              background: 'radial-gradient(circle, rgba(253,224,71,0.18) 0%, transparent 70%)',
            }}
            initial={{ scale: 0, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.6 }}
          />
        )}
      </AnimatePresence>

      {/* Logo */}
      <motion.div
        className="absolute top-6 left-1/2 -translate-x-1/2 flex items-center gap-3"
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.8 }}
      >
        <img src={logoSrc} alt="WinBig Africa" className="h-10 object-contain" />
        <span className="text-white font-bold text-lg tracking-widest uppercase">WinBig Africa</span>
      </motion.div>

      {/* Prize image — small corner badge */}
      <AnimatePresence>
        {(stage === 'celebrating') && (
          <motion.div
            className="absolute top-20 right-8 w-28 rounded-xl overflow-hidden border-2 border-yellow-400 shadow-xl"
            initial={{ opacity: 0, scale: 0.5, rotate: 10 }}
            animate={{ opacity: 1, scale: 1, rotate: 0 }}
            transition={{ delay: 0.5, type: 'spring', stiffness: 200 }}
          >
            <img src={prizeSrc} alt="Prize" className="w-full object-cover" />
          </motion.div>
        )}
      </AnimatePresence>

      {/* Main content */}
      <div className="relative z-10 flex flex-col items-center gap-8 px-8 text-center">

        {/* LOADING */}
        {stage === 'loading' && (
          <motion.div
            className="flex flex-col items-center gap-4"
            initial={{ opacity: 0 }} animate={{ opacity: 1 }}
          >
            <div className="w-12 h-12 border-4 border-yellow-400 border-t-transparent rounded-full animate-spin" />
            <p className="text-yellow-200 text-lg tracking-widest uppercase">Loading Draw Results…</p>
          </motion.div>
        )}

        {/* READY */}
        {stage === 'ready' && (
          <motion.div
            className="flex flex-col items-center gap-8"
            initial={{ opacity: 0, scale: 0.9 }} animate={{ opacity: 1, scale: 1 }}
            transition={{ duration: 0.6 }}
          >
            {error ? (
              <p className="text-red-400 text-xl">{error}</p>
            ) : !winner ? (
              <p className="text-yellow-200 text-2xl">No draw results available yet.</p>
            ) : (
              <>
                <div className="space-y-2">
                  <p className="text-yellow-400 text-sm font-semibold tracking-[0.3em] uppercase">
                    Draw #{winner.draw_number} · {winner.game_name}
                  </p>
                  <h1
                    className="text-5xl md:text-7xl font-black tracking-tight"
                    style={{ background: 'linear-gradient(135deg, #fde047, #f59e0b, #fbbf24)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}
                  >
                    WINNER REVEAL
                  </h1>
                  <p className="text-white/60 text-lg">Press the button when ready to announce</p>
                </div>

                <motion.img
                  src={prizeSrc}
                  alt="iPhone 17 Pro Max"
                  className="w-48 h-48 object-cover rounded-2xl border-2 border-yellow-400/50 shadow-2xl"
                  animate={{ y: [0, -8, 0] }}
                  transition={{ duration: 3, repeat: Infinity, ease: 'easeInOut' }}
                />

                <motion.button
                  className="px-12 py-5 text-2xl font-black tracking-widest uppercase rounded-full text-black shadow-2xl"
                  style={{ background: 'linear-gradient(135deg, #fde047, #f59e0b)' }}
                  whileHover={{ scale: 1.06 }}
                  whileTap={{ scale: 0.96 }}
                  onClick={startReveal}
                >
                  Reveal Winner
                </motion.button>
              </>
            )}
          </motion.div>
        )}

        {/* COUNTDOWN */}
        {stage === 'countdown' && (
          <AnimatePresence mode="wait">
            <motion.div
              key={countdownNum}
              className="flex flex-col items-center gap-4"
              initial={{ scale: 2, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.5, opacity: 0 }}
              transition={{ duration: 0.4, type: 'spring', stiffness: 200 }}
            >
              <span
                className="text-[clamp(120px,30vw,240px)] font-black leading-none"
                style={{ background: 'linear-gradient(135deg, #fde047, #f59e0b)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}
              >
                {countdownNum}
              </span>
              <p className="text-white/50 text-2xl tracking-widest uppercase">Get ready…</p>
            </motion.div>
          </AnimatePresence>
        )}

        {/* ROLLING */}
        {stage === 'rolling' && (
          <motion.div
            className="flex flex-col items-center gap-6"
            initial={{ opacity: 0 }} animate={{ opacity: 1 }}
          >
            <p className="text-yellow-400 text-sm font-semibold tracking-[0.3em] uppercase">
              Drawing winner…
            </p>
            <div
              className="px-8 py-5 rounded-2xl border border-green-400/30 font-mono"
              style={{ background: 'rgba(0,255,100,0.04)' }}
            >
              <div className="text-[clamp(32px,8vw,72px)] tracking-[0.15em] leading-none">
                {rollingChars.map((c, i) => (
                  <SlotChar key={i} char={c} locked={i < lockedCount} />
                ))}
              </div>
            </div>
          </motion.div>
        )}

        {/* REVEAL */}
        {stage === 'reveal' && winner && (
          <motion.div
            className="flex flex-col items-center gap-6"
            initial={{ scale: 0.4, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            transition={{ type: 'spring', stiffness: 180, damping: 14 }}
          >
            <p className="text-yellow-400 text-sm font-semibold tracking-[0.3em] uppercase">
              Winner Ticket
            </p>
            <div
              className="px-10 py-6 rounded-2xl font-mono"
              style={{
                background: 'linear-gradient(135deg, rgba(253,224,71,0.12), rgba(245,158,11,0.08))',
                border: '2px solid rgba(253,224,71,0.6)',
                boxShadow: '0 0 60px rgba(253,224,71,0.25)',
              }}
            >
              <span
                className="text-[clamp(36px,9vw,80px)] font-black tracking-[0.15em] leading-none"
                style={{ color: '#fde047', textShadow: '0 0 40px rgba(253,224,71,0.8)' }}
              >
                {winner.serial_number}
              </span>
            </div>
          </motion.div>
        )}

        {/* CELEBRATING */}
        {stage === 'celebrating' && winner && (
          <motion.div
            className="flex flex-col items-center gap-6"
            initial={{ opacity: 0 }} animate={{ opacity: 1 }}
            transition={{ duration: 0.4 }}
          >
            <motion.p
              className="text-5xl md:text-7xl font-black"
              style={{ background: 'linear-gradient(135deg, #fde047, #f59e0b, #fbbf24)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}
              animate={{ scale: [1, 1.04, 1] }}
              transition={{ duration: 2, repeat: Infinity }}
            >
              WE HAVE A WINNER!
            </motion.p>

            <div
              className="px-10 py-6 rounded-2xl font-mono"
              style={{
                background: 'linear-gradient(135deg, rgba(253,224,71,0.14), rgba(245,158,11,0.08))',
                border: '2px solid rgba(253,224,71,0.7)',
                boxShadow: '0 0 80px rgba(253,224,71,0.3)',
              }}
            >
              <span
                className="text-[clamp(36px,9vw,80px)] font-black tracking-[0.15em] leading-none"
                style={{ color: '#fde047', textShadow: '0 0 40px rgba(253,224,71,0.9)' }}
              >
                {winner.serial_number}
              </span>
            </div>

            <div className="space-y-2 text-center">
              {winner.customer_name && (
                <p className="text-white text-2xl font-semibold">{winner.customer_name}</p>
              )}
              {winner.customer_phone && (
                <p className="text-white/60 text-xl font-mono tracking-widest">
                  {maskPhone(winner.customer_phone)}
                </p>
              )}
              <p className="text-yellow-400/80 text-base tracking-widest uppercase mt-2">
                {winner.game_name} · Draw #{winner.draw_number}
              </p>
            </div>

            <div className="mt-4 space-y-1 text-center">
              <p className="text-white/40 text-sm tracking-widest uppercase">Prize</p>
              <p
                className="text-3xl font-black"
                style={{ background: 'linear-gradient(135deg, #ffffff, #fde047)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}
              >
                iPhone 17 Pro Max
              </p>
            </div>

            <motion.button
              className="mt-6 px-8 py-3 rounded-full border border-white/20 text-white/50 text-sm tracking-widest uppercase hover:border-white/40 hover:text-white/70 transition-colors"
              onClick={() => { fireConfetti(); fireConfetti() }}
              whileTap={{ scale: 0.95 }}
            >
              Celebrate Again 🎊
            </motion.button>
          </motion.div>
        )}
      </div>

      {/* Bottom NLA notice */}
      <motion.p
        className="absolute bottom-4 text-white/20 text-xs tracking-widest uppercase"
        initial={{ opacity: 0 }} animate={{ opacity: 1 }} transition={{ delay: 1 }}
      >
        Licensed by the National Lottery Authority · Ghana
      </motion.p>
    </div>
  )
}
