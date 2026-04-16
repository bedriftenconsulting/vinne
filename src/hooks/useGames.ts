import { useState, useEffect } from 'react';
import { apiClient, type Game } from '@/lib/api';
import type { Competition } from '@/lib/competitions';

// ── Helpers ───────────────────────────────────────────────────────────────────

// Build a draw end time from the game's draw_time ("HH:MM") for today/tomorrow
function nextDrawTime(drawTime: string): Date {
  const [hh, mm] = drawTime.split(':').map(Number);
  const now = new Date();
  const d = new Date(now);
  d.setHours(hh, mm, 0, 0);
  // If already past today's draw time, use tomorrow
  if (d.getTime() <= now.getTime()) d.setDate(d.getDate() + 1);
  return d;
}

function gameToCompetition(g: Game): Competition {
  // Resolve end time: prefer explicit end_date, else compute from draw_time
  let endsAt: Date;
  if (g.end_date) {
    endsAt = new Date(g.end_date);
  } else if (g.draw_date) {
    endsAt = new Date(g.draw_date);
  } else if (g.draw_time) {
    endsAt = nextDrawTime(g.draw_time);
  } else {
    endsAt = new Date(Date.now() + 24 * 60 * 60 * 1000);
  }

  const msLeft = endsAt.getTime() - Date.now();
  const totalTickets = g.total_tickets ?? 1000;
  const soldTickets  = g.sold_tickets  ?? 0;
  const pct = totalTickets > 0 ? soldTickets / totalTickets : 0;

  let tag: Competition['tag'] = 'LIVE';
  if (pct >= 1 || msLeft <= 0) tag = 'Sold Out';
  else if (msLeft < 2 * 60 * 60 * 1000) tag = 'Ending Soon';

  // Price: base_price is in GHS (not pesewas), ticket_price fallback in pesewas
  const priceGHS = g.base_price ?? (g.ticket_price ? g.ticket_price / 100 : 20);

  // Normalise logo URL — replace internal MinIO host or localhost:port with relative path
  const rawImage = g.image_url || g.logo_url || '';
  const image = rawImage
    .replace(/^https?:\/\/minio:\d+\//, '/')
    .replace(/^https?:\/\/localhost:\d+\//, '/');

  return {
    id: g.id,
    title: g.name,
    image,
    ticketPrice: priceGHS,
    currency: g.currency || 'GHS',
    totalTickets,
    soldTickets,
    endsAt,
    tag,
    featured: false,
    description: g.description || g.prize_description || '',
    maxTicketsPerPlayer: g.max_tickets_per_player ?? undefined,
  };
}

function pickFeatured(list: Competition[]): Competition {
  const active = list.filter(c => c.tag === 'LIVE' || c.tag === 'Ending Soon');
  const pool = active.length > 0 ? active : list;
  return pool.sort((a, b) => a.endsAt.getTime() - b.endsAt.getTime())[0];
}

// ── Hook ──────────────────────────────────────────────────────────────────────

export interface UseGamesResult {
  competitions: Competition[];
  featured: Competition | null;
  loading: boolean;
  error: string | null;
  isReal: boolean;
}

const EMPTY: UseGamesResult = { competitions: [], featured: null, loading: true, error: null, isReal: false };

const POLL_INTERVAL = 30_000; // 30 seconds

export function useGames(): UseGamesResult {
  const [result, setResult] = useState<UseGamesResult>(EMPTY);

  const fetchGames = () => {
    apiClient.getActiveGames()
      .then((games) => {
        const activeGames = (games || []).filter(
          g => g.status?.toUpperCase() === 'ACTIVE'
        );
        if (activeGames.length === 0) {
          setResult(prev => ({ ...prev, competitions: [], featured: null, loading: false, error: null, isReal: true }));
          return;
        }
        const mapped = activeGames.map(gameToCompetition);
        const featured = pickFeatured(mapped);
        featured.featured = true;
        setResult({ competitions: mapped, featured, loading: false, error: null, isReal: true });
      })
      .catch((err) => {
        setResult(prev => ({ ...prev, loading: false, error: err.message }));
      });
  };

  useEffect(() => {
    fetchGames();
    const timer = setInterval(fetchGames, POLL_INTERVAL);
    return () => clearInterval(timer);
  }, []);

  return result;
}
