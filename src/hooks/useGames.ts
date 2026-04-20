import { useState, useEffect } from 'react';
import { apiClient, type Game } from '@/lib/api';
import type { Competition } from '@/lib/competitions';

// ── Helpers ───────────────────────────────────────────────────────────────────

// Build a draw end time from the game's draw_time ("HH:MM") for today/tomorrow
function nextDrawTime(drawTime: string): Date {
  // Extract HH:MM from various formats: "20:00", "0001-01-01 20:00:00 BC", etc.
  const match = drawTime.match(/(\d{1,2}):(\d{2})/);
  if (!match) return new Date(Date.now() + 24 * 60 * 60 * 1000);
  const [, hh, mm] = match;
  const now = new Date();
  const d = new Date(now);
  d.setHours(parseInt(hh), parseInt(mm), 0, 0);
  // If already past today's draw time, use tomorrow
  if (d.getTime() <= now.getTime()) d.setDate(d.getDate() + 1);
  return d;
}

function gameToCompetition(g: Game): Competition {
  const isSpecial = g.draw_frequency === 'special' || g.draw_frequency === 'SPECIAL';

  // Resolve end time
  let endsAt: Date;
  if (isSpecial) {
    // Special draws: always use the explicit end_date + draw_time — never the 24h rolling cycle
    if (g.end_date) {
      const dateStr = g.end_date.split('T')[0];
      // Extract HH:MM from draw_time — handles "20:00", "0001-01-01 20:00:00 BC", etc.
      const timeMatch = g.draw_time?.match(/(\d{1,2}):(\d{2})/);
      if (timeMatch) {
        const hh = timeMatch[1].padStart(2, '0');
        const mm = timeMatch[2];
        endsAt = new Date(`${dateStr}T${hh}:${mm}:00`);
      } else {
        // No draw_time — use end of day on end_date (local time)
        const [y, mo, d] = dateStr.split('-').map(Number);
        endsAt = new Date(y, mo - 1, d, 20, 0, 0);
      }
    } else if (g.draw_date) {
      endsAt = new Date(g.draw_date);
    } else {
      endsAt = new Date(Date.now() + 365 * 24 * 60 * 60 * 1000);
    }
  } else if (g.draw_date) {
    endsAt = new Date(g.draw_date);
  } else if (g.draw_time) {
    // All other frequencies: rolling 24h countdown to next draw time
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
    const API_BASE = import.meta.env.PROD
      ? (import.meta.env.VITE_API_URL as string) || 'https://api.winbigafrica.com'
      : '';
    Promise.all([
      apiClient.getActiveGames(),
      fetch(`${API_BASE}/api/v1/players/games/schedules/weekly`).then(r => r.json()).catch(() => null),
    ])
      .then(([games, scheduleData]) => {
        const activeGames = (games || []).filter(
          g => g.status?.toUpperCase() === 'ACTIVE'
        );
        if (activeGames.length === 0) {
          setResult(prev => ({ ...prev, competitions: [], featured: null, loading: false, error: null, isReal: true }));
          return;
        }

        // Build a map of game_id → sold_tickets from the schedule response
        const soldMap: Record<string, number> = {};
        const schedules: any[] = scheduleData?.data?.schedules || [];
        for (const s of schedules) {
          if (s.game_id && typeof s.sold_tickets === 'number') {
            // Use max in case there are multiple schedules for the same game this week
            soldMap[s.game_id] = Math.max(soldMap[s.game_id] ?? 0, s.sold_tickets);
          }
        }

        const mapped = activeGames.map(g => {
          const comp = gameToCompetition(g);
          if (soldMap[g.id] !== undefined) comp.soldTickets = soldMap[g.id];
          return comp;
        });
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
