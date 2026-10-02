const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export interface PortfolioPosition {
  asset_id: string;
  ticker: string;
  name: string;
  asset_class: string;
  currency: string;
  quantity: string;
  average_price: string;
  total_cost: string;
  current_price: string;
  market_value: string;
  profit_loss: string;
  profit_loss_pct: string;
  price_updated_at?: string;
}

export interface ClassAllocation {
  asset_class: string;
  total_cost: string;
  market_value: string;
  weight_pct: string;
}

export interface PortfolioSummary {
  total_cost: string;
  market_value: string;
  profit_loss: string;
  profit_loss_pct: string;
  positions: PortfolioPosition[];
  allocations: ClassAllocation[];
  updated_at: string;
}

export interface HistoryPoint {
  date: string;
  total_cost: string;
  market_value: string;
  cdi_rate?: string;
  selic_rate?: string;
  ipca_rate?: string;
}

export interface MacroBenchmark {
  series_name: string;
  reference_date: string;
  rate_value: string;
}

export interface MonthCard {
  year_month: string;
  month_number: number;
  total_sales: string;
  taxable_base: string;
  tax_due: string;
  final_darf: string;
  status: "DUE" | "EXEMPT" | "NO_ACTIVITY";
  buckets: any[];
}

export interface MonthlyPreviewResponse {
  year: string;
  total_tax_due: string;
  total_darf: string;
  months: MonthCard[];
}

export interface ParsedTransaction {
  operation_type: string;
  ticker: string;
  asset_description: string;
  asset_class: string;
  quantity: string;
  unit_price: string;
  total_amount: string;
  costs: string;
  is_day_trade: boolean;
}

export interface ParsedBrokerageNote {
  note_number: string;
  broker_name: string;
  operation_date: string;
  settlement_date?: string;
  total_costs: string;
  gross_amount: string;
  net_amount: string;
  irrf: string;
  transactions: ParsedTransaction[];
}

export async function fetchPortfolioSummary(): Promise<PortfolioSummary> {
  const res = await fetch(`${API_BASE}/api/v1/portfolio/summary`, { cache: "no-store" });
  if (!res.ok) throw new Error("Failed to fetch portfolio summary");
  return res.json();
}

export async function fetchPortfolioHistory(): Promise<HistoryPoint[]> {
  const res = await fetch(`${API_BASE}/api/v1/portfolio/history`, { cache: "no-store" });
  if (!res.ok) throw new Error("Failed to fetch portfolio history");
  return res.json();
}

export async function fetchBenchmarks(startDate?: string, endDate?: string): Promise<MacroBenchmark[]> {
  let url = `${API_BASE}/api/v1/benchmarks`;
  const params = new URLSearchParams();
  if (startDate) params.set("start_date", startDate);
  if (endDate) params.set("end_date", endDate);
  if (params.toString()) url += `?${params.toString()}`;

  const res = await fetch(url, { cache: "no-store" });
  if (!res.ok) throw new Error("Failed to fetch benchmarks");
  return res.json();
}

export async function fetchMonthlyTax(year: string): Promise<MonthlyPreviewResponse> {
  const res = await fetch(`${API_BASE}/api/v1/tax/monthly?year=${year}`, { cache: "no-store" });
  if (!res.ok) throw new Error("Failed to fetch monthly tax");
  return res.json();
}

export async function fetchTaxDrilldown(yearMonth: string, bucket: string): Promise<any> {
  const res = await fetch(`${API_BASE}/api/v1/tax/drilldown?year_month=${yearMonth}&bucket=${bucket}`, { cache: "no-store" });
  if (!res.ok) throw new Error("Failed to fetch tax drilldown");
  return res.json();
}

export async function parseBrokerageNote(file: File): Promise<ParsedBrokerageNote> {
  const formData = new FormData();
  formData.append("file", file);

  const res = await fetch(`${API_BASE}/api/v1/documents/parse`, {
    method: "POST",
    body: formData,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "Erro ao processar PDF" }));
    throw new Error(err.error || "Erro ao processar PDF");
  }
  return res.json();
}

export async function confirmBrokerageNote(payload: any, file?: File): Promise<any> {
  if (file) {
    const formData = new FormData();
    formData.append("file", file);
    formData.append("data", JSON.stringify(payload));
    const res = await fetch(`${API_BASE}/api/v1/documents/confirm`, {
      method: "POST",
      body: formData,
    });
    if (!res.ok) throw new Error("Failed to confirm note");
    return res.json();
  }

  const res = await fetch(`${API_BASE}/api/v1/documents/confirm`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error("Failed to confirm note");
  return res.json();
}

export async function fetchTransactions(): Promise<any[]> {
  const res = await fetch(`${API_BASE}/api/v1/transactions`, { cache: "no-store" });
  if (!res.ok) throw new Error("Failed to fetch transactions");
  return res.json();
}
