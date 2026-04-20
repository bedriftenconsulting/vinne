// Central API helper for the website
const BASE = import.meta.env.VITE_API_URL || "/api/v1";

export const getToken = () => localStorage.getItem("player_token");
export const getPlayerId = (): string | null => {
  const t = getToken();
  if (!t) return null;
  try {
    const p = JSON.parse(atob(t.split(".")[1]));
    return p.user_id || p.sub || null;
  } catch { return null; }
};

const authHeaders = () => ({
  "Content-Type": "application/json",
  Authorization: `Bearer ${getToken()}`,
});

// ── Games ─────────────────────────────────────────────────────────────────────
export interface ApiGame {
  id: string;
  code: string;
  name: string;
  description: string;
  base_price: number;
  min_stake: number;
  max_stake: number;
  total_tickets: number;
  max_tickets_per_player: number;
  draw_date: string;
  draw_time: string;
  draw_frequency: string;
  logo_url: string;
  brand_color: string;
  prize_details: string; // JSON string
  status: string;
  sales_cutoff_minutes: number;
  number_range_min: number;
  number_range_max: number;
  selection_count: number;
}

export interface ApiSchedule {
  id: string;
  game_id: string;
  game_code: string;
  game_name: string;
  scheduled_start: string | { seconds: number };
  scheduled_end: string | { seconds: number };
  scheduled_draw: string | { seconds: number };
  draw_number?: number;
  status: string;
  is_active: boolean;
  logo_url?: string;
  brand_color?: string;
}

export const fetchActiveGames = async (): Promise<ApiGame[]> => {
  const r = await fetch(`${BASE}/players/games`);
  const d = await r.json();
  return d?.data?.games ?? [];
};

export const fetchGameSchedule = async (gameId: string): Promise<ApiSchedule[]> => {
  const r = await fetch(`${BASE}/players/games/${gameId}/schedule`);
  const d = await r.json();
  return d?.data?.schedules ?? [];
};

// ── Wallet ────────────────────────────────────────────────────────────────────
export const fetchWalletBalance = async (playerId: string): Promise<number> => {
  const r = await fetch(`${BASE}/players/${playerId}/wallet/balance`, { headers: authHeaders() });
  const d = await r.json();
  return d?.data?.balance ?? d?.balance ?? 0;
};

// ── Payments ──────────────────────────────────────────────────────────────────
export interface DepositPayload {
  amount: number;           // in pesewas
  mobile_money_phone: string;
  payment_method: string;   // MTN | TELECEL | AIRTELTIGO
  customer_name: string;
}

export const initiateDeposit = async (playerId: string, payload: DepositPayload) => {
  const r = await fetch(`${BASE}/players/${playerId}/deposit`, {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify(payload),
  });
  const d = await r.json();
  // Deposit may fail due to saga issues but we still proceed — record attempt
  return d;
};
export interface BuyTicketPayload {
  game_code: string;
  game_schedule_id: string;
  draw_number: number;
  bet_lines: { line_number: number; bet_type: string; total_amount: number }[];
  customer_phone?: string;
}

export const buyTicket = async (playerId: string, payload: BuyTicketPayload) => {
  const r = await fetch(`${BASE}/players/${playerId}/tickets`, {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify(payload),
  });
  const d = await r.json();
  if (!r.ok || d.error) throw new Error(d.error?.message || d.error || d.message || "Purchase failed");
  return d?.data ?? d;
};
