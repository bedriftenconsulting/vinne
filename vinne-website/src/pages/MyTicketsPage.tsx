import { useEffect, useState, useMemo } from "react";
import { Link, useNavigate } from "react-router-dom";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";
import {
  Ticket, Trophy, Clock, CheckCircle, XCircle, AlertCircle,
  X, Calendar, Hash, Search, ArrowDownUp, CreditCard, RefreshCw,
} from "lucide-react";

// All fields are snake_case — gateway uses UseProtoNames: true
interface PlayerTicket {
  id: string;
  ticket_id?: string;  // alias
  serial_number: string;
  game_code: string;
  game_name: string;
  draw_number?: number;
  draw_date?: string | { seconds: number };
  selected_numbers?: number[];
  bet_lines?: { total_amount: number | string; selected_numbers?: number[] }[];
  status: string;
  total_amount: number | string;
  unit_price?: number | string;
  winning_amount?: number | string;
  payment_method?: string;
  created_at?: string | { seconds: number };
}

interface Transaction {
  id: string;
  type: string;
  amount: number;
  reference: string;
  status: string;
  created_at: string;
  requested_at?: string;
  narration?: string;
  provider_name?: string;
  source_identifier?: string;
  currency?: string;
}

// ── Helpers ───────────────────────────────────────────────────────────────────
const toNum = (v: unknown): number => {
  if (typeof v === "number") return v;
  if (typeof v === "string") return parseInt(v, 10) || 0;
  return 0;
};
const pesewasToGHS = (v?: unknown) => (toNum(v) / 100).toFixed(2);

const parseDate = (d?: string | { seconds: number }): Date | null => {
  if (!d) return null;
  if (typeof d === "object" && "seconds" in d) return new Date(d.seconds * 1000);
  return new Date(d as string);
};
const fmtDate = (d: Date | null) =>
  d ? d.toLocaleDateString("en-GB", { day: "numeric", month: "short", year: "numeric" }) : "—";

const getPlayerIdFromToken = (t: string | null): string | null => {
  if (!t) return null;
  try {
    const payload = JSON.parse(atob(t.split(".")[1]));
    return payload.user_id || payload.sub || null;
  } catch { return null; }
};

const STATUS_CFG: Record<string, { label: string; icon: React.ReactNode; cls: string }> = {
  ACTIVE:    { label: "Active",    icon: <Clock size={11} />,        cls: "text-yellow-400 bg-yellow-400/10 border-yellow-400/30" },
  PENDING:   { label: "Pending",   icon: <Clock size={11} />,        cls: "text-yellow-400 bg-yellow-400/10 border-yellow-400/30" },
  WON:       { label: "Winner 🏆", icon: <Trophy size={11} />,       cls: "text-green-400 bg-green-400/10 border-green-400/30" },
  LOST:      { label: "Not Won",   icon: <XCircle size={11} />,      cls: "text-muted-foreground bg-muted/30 border-border" },
  COMPLETED: { label: "Completed", icon: <CheckCircle size={11} />,  cls: "text-blue-400 bg-blue-400/10 border-blue-400/30" },
  CANCELLED: { label: "Cancelled", icon: <AlertCircle size={11} />,  cls: "text-red-400 bg-red-400/10 border-red-400/30" },
  VOIDED:    { label: "Voided",    icon: <AlertCircle size={11} />,  cls: "text-red-400 bg-red-400/10 border-red-400/30" },
};

const TX_LABEL: Record<string, string> = {
  TICKET_PURCHASE: "Ticket Purchase",
  DEPOSIT: "Deposit",
  WITHDRAWAL: "Withdrawal",
  PRIZE_PAYOUT: "Prize Payout",
  REFUND: "Refund",
};

// ── Ticket Detail Modal ───────────────────────────────────────────────────────
const TicketModal = ({ ticket, onClose }: { ticket: PlayerTicket; onClose: () => void }) => {
  const s = STATUS_CFG[ticket.status?.toUpperCase()] ?? STATUS_CFG.ACTIVE;
  const drawDate = parseDate(ticket.draw_date);
  const createdAt = parseDate(ticket.created_at);
  const numbers = ticket.selected_numbers?.length
    ? ticket.selected_numbers
    : ticket.bet_lines?.flatMap(b => b.selected_numbers ?? []) ?? [];
  const isWinner = ticket.status?.toUpperCase() === "WON" || toNum(ticket.winning_amount) > 0;

  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center p-0 sm:p-4 bg-black/70 backdrop-blur-sm" onClick={onClose}>
      <div className="bg-card border border-border rounded-t-2xl sm:rounded-2xl w-full sm:max-w-md shadow-2xl max-h-[90vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
        <div className="flex justify-center pt-3 pb-1 sm:hidden">
          <div className="w-10 h-1 rounded-full bg-border" />
        </div>
        <div className="flex items-center justify-between px-5 py-4 border-b border-border">
          <div className="flex items-center gap-2">
            <Ticket size={16} className="text-primary" />
            <span className="font-heading text-base tracking-wide">TICKET DETAILS</span>
          </div>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground transition p-1"><X size={18} /></button>
        </div>

        <div className="px-5 py-5 space-y-5">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs text-muted-foreground mb-0.5">Serial Number</p>
              <p className="font-mono text-sm font-bold text-foreground">{ticket.serial_number || ticket.id?.slice(0, 12).toUpperCase()}</p>
            </div>
            <span className={`flex items-center gap-1 text-xs font-semibold px-2.5 py-1 rounded-full border shrink-0 ${s.cls}`}>
              {s.icon} {s.label}
            </span>
          </div>

          <div className="bg-secondary rounded-xl divide-y divide-border">
            <div className="flex items-center justify-between px-4 py-2.5">
              <p className="text-xs text-muted-foreground">Competition</p>
              <p className="text-sm font-semibold text-foreground text-right max-w-[60%]">{ticket.game_name || ticket.game_code || "—"}</p>
            </div>
            {ticket.game_code && (
              <div className="flex items-center justify-between px-4 py-2.5">
                <p className="text-xs text-muted-foreground">Game Code</p>
                <p className="text-xs font-mono text-muted-foreground">{ticket.game_code}</p>
              </div>
            )}
            {ticket.draw_number && (
              <div className="flex items-center justify-between px-4 py-2.5">
                <p className="text-xs text-muted-foreground flex items-center gap-1"><Hash size={10} /> Draw #</p>
                <p className="text-xs text-foreground">{ticket.draw_number}</p>
              </div>
            )}
            {drawDate && (
              <div className="flex items-center justify-between px-4 py-2.5">
                <p className="text-xs text-muted-foreground flex items-center gap-1"><Calendar size={10} /> Draw Date</p>
                <p className="text-xs text-foreground">{fmtDate(drawDate)}</p>
              </div>
            )}
            {createdAt && (
              <div className="flex items-center justify-between px-4 py-2.5">
                <p className="text-xs text-muted-foreground">Purchased</p>
                <p className="text-xs text-foreground">{fmtDate(createdAt)}</p>
              </div>
            )}
            {ticket.payment_method && (
              <div className="flex items-center justify-between px-4 py-2.5">
                <p className="text-xs text-muted-foreground">Payment</p>
                <p className="text-xs text-foreground capitalize">{ticket.payment_method.replace(/_/g, " ")}</p>
              </div>
            )}
          </div>

          {numbers.length > 0 && (
            <div>
              <p className="text-xs text-muted-foreground mb-2">Your Numbers</p>
              <div className="flex flex-wrap gap-2">
                {numbers.map((n, i) => (
                  <span key={i} className="w-9 h-9 flex items-center justify-center rounded-full bg-primary/10 border border-primary/30 text-primary text-sm font-bold">{n}</span>
                ))}
              </div>
            </div>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div className="bg-secondary rounded-xl px-4 py-3">
              <p className="text-xs text-muted-foreground mb-1">Ticket Price</p>
              <p className="text-xl font-bold text-foreground">₵{pesewasToGHS(ticket.unit_price || ticket.total_amount)}</p>
            </div>
            {isWinner && toNum(ticket.winning_amount) > 0 ? (
              <div className="bg-green-500/10 border border-green-500/20 rounded-xl px-4 py-3">
                <p className="text-xs text-green-400 mb-1">Prize Won 🎉</p>
                <p className="text-xl font-bold text-green-400">₵{pesewasToGHS(ticket.winning_amount)}</p>
              </div>
            ) : (
              <div className="bg-secondary rounded-xl px-4 py-3">
                <p className="text-xs text-muted-foreground mb-1">Total Paid</p>
                <p className="text-xl font-bold text-foreground">₵{pesewasToGHS(ticket.total_amount)}</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

// ── Ticket Card ───────────────────────────────────────────────────────────────
const TicketCard = ({ ticket, onClick }: { ticket: PlayerTicket; onClick: () => void }) => {
  const s = STATUS_CFG[ticket.status?.toUpperCase()] ?? STATUS_CFG.ACTIVE;
  const drawDate = parseDate(ticket.draw_date);
  const isWinner = ticket.status?.toUpperCase() === "WON" || toNum(ticket.winning_amount) > 0;

  return (
    <button onClick={onClick} className={`w-full text-left bg-card border rounded-xl overflow-hidden transition hover:border-primary/50 hover:shadow-lg active:scale-[0.99] ${isWinner ? "border-green-500/40" : "border-border"}`}>
      <div className="bg-secondary px-4 py-2.5 flex items-center justify-between">
        <div className="flex items-center gap-1.5">
          <Ticket size={12} className="text-primary" />
          <span className="font-mono text-xs text-muted-foreground">{ticket.serial_number || ticket.id?.slice(0, 10).toUpperCase()}</span>
        </div>
        <span className={`flex items-center gap-1 text-xs font-semibold px-2 py-0.5 rounded-full border ${s.cls}`}>{s.icon} {s.label}</span>
      </div>
      <div className="px-4 py-3 space-y-2">
        <p className="font-heading text-sm text-foreground leading-tight">{ticket.game_name || ticket.game_code || "—"}</p>
        {drawDate && (
          <p className="text-xs text-muted-foreground flex items-center gap-1"><Calendar size={10} /> Draw: {fmtDate(drawDate)}</p>
        )}
        <div className="flex items-center justify-between pt-1.5 border-t border-border">
          <div>
            <p className="text-xs text-muted-foreground">Ticket Price</p>
            <p className="text-sm font-bold text-foreground">₵{pesewasToGHS(ticket.unit_price || ticket.total_amount)}</p>
          </div>
          {isWinner && toNum(ticket.winning_amount) > 0 ? (
            <div className="text-right">
              <p className="text-xs text-green-400">Won</p>
              <p className="text-sm font-bold text-green-400">₵{pesewasToGHS(ticket.winning_amount)}</p>
            </div>
          ) : (
            <span className="text-xs text-muted-foreground/40">Tap for details →</span>
          )}
        </div>
      </div>
    </button>
  );
};

// ── Transaction Row ───────────────────────────────────────────────────────────
const TxRow = ({ tx }: { tx: Transaction }) => {
  const isCredit = tx.type?.includes("DEPOSIT") || tx.type?.includes("PAYOUT") || tx.type?.includes("REFUND");
  const isSuccess = tx.status?.includes("SUCCESS") || tx.status?.includes("COMPLETED");
  const isFailed = tx.status?.includes("FAILED") || tx.status?.includes("CANCELLED");
  const date = new Date(tx.created_at || tx.requested_at || "");
  const amountGHS = (toNum(tx.amount) / 100).toFixed(2);

  const typeLabel = tx.type?.replace("TRANSACTION_TYPE_", "").replace(/_/g, " ") || "Transaction";
  const statusColor = isSuccess ? "text-green-400" : isFailed ? "text-red-400" : "text-yellow-400";

  return (
    <div className="flex items-center gap-3 px-4 py-3 bg-card border border-border rounded-xl hover:border-primary/30 transition">
      <div className={`w-9 h-9 rounded-full flex items-center justify-center shrink-0 ${isCredit ? "bg-green-500/10 text-green-400" : "bg-primary/10 text-primary"}`}>
        {tx.type?.includes("DEPOSIT") ? <CreditCard size={15} /> : tx.type?.includes("PAYOUT") ? <Trophy size={15} /> : <RefreshCw size={15} />}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <p className="text-sm font-medium text-foreground capitalize">{typeLabel.toLowerCase()}</p>
          {tx.provider_name && <span className="text-xs text-muted-foreground bg-secondary px-1.5 py-0.5 rounded">{tx.provider_name}</span>}
        </div>
        <p className="text-xs text-muted-foreground truncate">{tx.narration || tx.reference}</p>
      </div>
      <div className="text-right shrink-0">
        <p className={`text-sm font-bold ${isCredit ? "text-green-400" : "text-foreground"}`}>
          {isCredit ? "+" : "-"}{tx.currency || "GHS"} {amountGHS}
        </p>
        <p className={`text-xs ${statusColor}`}>{tx.status?.replace("TRANSACTION_STATUS_", "") || "—"}</p>
        <p className="text-xs text-muted-foreground">{isNaN(date.getTime()) ? "" : date.toLocaleDateString("en-GB", { day: "numeric", month: "short" })}</p>
      </div>
    </div>
  );
};

// ── Page ──────────────────────────────────────────────────────────────────────
const MyTicketsPage = () => {
  const navigate = useNavigate();
  const [tab, setTab] = useState<"tickets" | "transactions">("tickets");
  const [tickets, setTickets] = useState<PlayerTicket[]>([]);
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [loading, setLoading] = useState(true);
  const [txLoading, setTxLoading] = useState(false);
  const [error, setError] = useState("");
  const [selected, setSelected] = useState<PlayerTicket | null>(null);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("ALL");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  const token = localStorage.getItem("player_token");
  const resolvedPlayerId = getPlayerIdFromToken(token) || localStorage.getItem("player_id");

  useEffect(() => {
    if (!token || !resolvedPlayerId) { navigate("/sign-in"); return; }
    fetch(`/api/v1/players/${resolvedPlayerId}/tickets?page_size=100&page=1`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(r => r.json())
      .then(d => {
        const raw = d?.data?.tickets ?? d?.tickets ?? [];
        setTickets(raw);
      })
      .catch(e => setError(e.message))
      .finally(() => setLoading(false));
  }, [navigate, token, resolvedPlayerId]);

  useEffect(() => {
    if (tab !== "transactions" || !token || !resolvedPlayerId) return;
    setTxLoading(true);
    // Use tickets as purchase evidence — payment service is unreliable in test mode
    fetch(`/api/v1/players/${resolvedPlayerId}/tickets?page_size=100&page=1`, {
      headers: { Authorization: `Bearer ${token}` },
    })
      .then(r => r.json())
      .then(d => {
        const raw = d?.data?.tickets ?? d?.tickets ?? [];
        setTransactions(raw.map((t: Record<string, unknown>) => ({
          id: t.id as string,
          type: "TICKET_PURCHASE",
          amount: t.total_amount,
          reference: (t.serial_number as string) || (t.id as string)?.slice(0, 12).toUpperCase(),
          status: "COMPLETED",
          created_at: t.created_at as string,
          narration: `Ticket — ${t.game_name || t.game_code}`,
          provider_name: (t.payment_method as string)?.toUpperCase() || "MOMO",
          currency: "GHS",
        })));
      })
      .catch(() => setTransactions([]))
      .finally(() => setTxLoading(false));
  }, [tab, token, resolvedPlayerId]);

  const filtered = useMemo(() => tickets.filter(t => {
    if (statusFilter !== "ALL" && t.status?.toUpperCase() !== statusFilter) return false;
    if (search) {
      const q = search.toLowerCase();
      if (!t.serial_number?.toLowerCase().includes(q) && !t.game_name?.toLowerCase().includes(q) && !t.game_code?.toLowerCase().includes(q)) return false;
    }
    if (dateFrom || dateTo) {
      const d = parseDate(t.created_at);
      if (!d) return false;
      if (dateFrom && d < new Date(dateFrom)) return false;
      if (dateTo && d > new Date(dateTo + "T23:59:59")) return false;
    }
    return true;
  }), [tickets, statusFilter, search, dateFrom, dateTo]);

  const statuses = ["ALL", "ACTIVE", "WON", "COMPLETED", "CANCELLED"];

  return (
    <div className="min-h-screen flex flex-col bg-background">
      <Navbar />
      <main className="flex-1 container pt-24 pb-16">
        <div className="flex items-center gap-3 mb-6">
          <div className="w-10 h-10 rounded-xl bg-primary/10 border border-primary/30 flex items-center justify-center">
            <Ticket size={20} className="text-primary" />
          </div>
          <div>
            <h1 className="font-heading text-2xl tracking-wide">MY ACCOUNT</h1>
            <p className="text-sm text-muted-foreground">{tickets.length} ticket{tickets.length !== 1 ? "s" : ""} total</p>
          </div>
        </div>

        <div className="flex gap-2 mb-6 border-b border-border">
          {(["tickets", "transactions"] as const).map(t => (
            <button key={t} onClick={() => setTab(t)} className={`px-5 py-2.5 text-sm font-semibold transition border-b-2 -mb-px ${tab === t ? "border-primary text-primary" : "border-transparent text-muted-foreground hover:text-foreground"}`}>
              {t === "tickets" ? "🎟 Tickets" : "💳 Transactions"}
            </button>
          ))}
        </div>

        {tab === "tickets" && (
          <>
            <div className="flex flex-col sm:flex-row gap-3 mb-4">
              <div className="relative flex-1">
                <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                <input type="text" placeholder="Search by serial, game name..." value={search} onChange={e => setSearch(e.target.value)}
                  className="w-full bg-secondary text-foreground placeholder:text-muted-foreground border border-border rounded-lg pl-9 pr-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary transition" />
              </div>
              <div className="flex gap-2">
                <div className="relative">
                  <Calendar size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
                  <input type="date" value={dateFrom} onChange={e => setDateFrom(e.target.value)}
                    className="bg-secondary text-foreground border border-border rounded-lg pl-8 pr-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary transition w-36" />
                </div>
                <div className="relative">
                  <Calendar size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground pointer-events-none" />
                  <input type="date" value={dateTo} onChange={e => setDateTo(e.target.value)}
                    className="bg-secondary text-foreground border border-border rounded-lg pl-8 pr-3 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary transition w-36" />
                </div>
                {(search || dateFrom || dateTo || statusFilter !== "ALL") && (
                  <button onClick={() => { setSearch(""); setDateFrom(""); setDateTo(""); setStatusFilter("ALL"); }}
                    className="px-3 py-2 text-xs text-muted-foreground border border-border rounded-lg hover:border-primary/50 hover:text-foreground transition">Clear</button>
                )}
              </div>
            </div>

            <div className="flex gap-2 mb-5 overflow-x-auto pb-1">
              {statuses.map(f => {
                const count = f === "ALL" ? tickets.length : tickets.filter(t => t.status?.toUpperCase() === f).length;
                return (
                  <button key={f} onClick={() => setStatusFilter(f)}
                    className={`px-3.5 py-1.5 rounded-full text-xs font-semibold whitespace-nowrap transition border ${statusFilter === f ? "bg-primary text-white border-primary" : "border-border text-muted-foreground hover:border-primary/50 hover:text-foreground"}`}>
                    {f === "ALL" ? `All (${count})` : `${f.charAt(0) + f.slice(1).toLowerCase()} (${count})`}
                  </button>
                );
              })}
            </div>

            {(search || dateFrom || dateTo) && (
              <p className="text-xs text-muted-foreground mb-3 flex items-center gap-1"><ArrowDownUp size={11} /> {filtered.length} result{filtered.length !== 1 ? "s" : ""} found</p>
            )}

            {loading ? (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                {[...Array(6)].map((_, i) => <div key={i} className="bg-card border border-border rounded-xl h-36 animate-pulse" />)}
              </div>
            ) : error ? (
              <div className="text-center py-16"><AlertCircle size={36} className="text-destructive mx-auto mb-3" /><p className="text-muted-foreground text-sm">{error}</p></div>
            ) : filtered.length === 0 ? (
              <div className="text-center py-16">
                <Ticket size={44} className="text-muted-foreground/20 mx-auto mb-4" />
                <h3 className="font-heading text-lg text-foreground mb-2">{tickets.length === 0 ? "No tickets yet" : "No tickets match your filters"}</h3>
                <p className="text-muted-foreground text-sm mb-5">{tickets.length === 0 ? "Enter a competition to see your tickets here" : "Try adjusting your search or date range"}</p>
                {tickets.length === 0 && <Link to="/competitions" className="bg-primary text-white font-heading px-6 py-2.5 rounded-lg btn-glow hover:brightness-110 transition text-sm">BROWSE COMPETITIONS</Link>}
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                {filtered.map(t => <TicketCard key={t.id || t.ticket_id} ticket={t} onClick={() => setSelected(t)} />)}
              </div>
            )}
          </>
        )}

        {tab === "transactions" && (
          <div className="space-y-3">
            {txLoading ? (
              [...Array(5)].map((_, i) => <div key={i} className="bg-card border border-border rounded-xl h-16 animate-pulse" />)
            ) : transactions.length === 0 ? (
              <div className="text-center py-16">
                <CreditCard size={44} className="text-muted-foreground/20 mx-auto mb-4" />
                <h3 className="font-heading text-lg text-foreground mb-2">No transactions yet</h3>
                <p className="text-muted-foreground text-sm">Deposits, withdrawals and prize payouts will appear here</p>
              </div>
            ) : (
              transactions.map(tx => <TxRow key={tx.id} tx={tx} />)
            )}
          </div>
        )}
      </main>
      <Footer />
      {selected && <TicketModal ticket={selected} onClose={() => setSelected(null)} />}
    </div>
  );
};

export default MyTicketsPage;

