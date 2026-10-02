"use client";

import { useEffect, useState } from "react";
import { Header } from "@/components/header";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { PerformanceChart } from "@/components/dashboard/performance-chart";
import { fetchPortfolioSummary, fetchPortfolioHistory, PortfolioSummary, HistoryPoint } from "@/lib/api";
import { formatCurrency, formatPercent } from "@/lib/utils";
import { TrendingUp, TrendingDown, DollarSign, Layers, PieChart, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PieChart as RechartsPieChart, Pie, Cell, Tooltip, ResponsiveContainer } from "recharts";

const DONUT_COLORS = ["#71717a", "#a1a1aa", "#52525b", "#d4d4d8", "#3f3f46", "#e4e4e7"];

export default function DashboardPage() {
  const [valuationMode, setValuationMode] = useState<"market" | "cost">("market");
  const [summary, setSummary] = useState<PortfolioSummary | null>(null);
  const [history, setHistory] = useState<HistoryPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  async function loadData() {
    try {
      setLoading(true);
      setError(null);
      const [sumData, histData] = await Promise.all([
        fetchPortfolioSummary().catch(() => null),
        fetchPortfolioHistory().catch(() => []),
      ]);
      setSummary(sumData);
      setHistory(histData || []);
    } catch (err: any) {
      setError(err?.message || "Erro ao carregar dados do portfólio");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadData();
  }, []);

  const totalCost = summary ? parseFloat(summary.total_cost) : 0;
  const marketValue = summary ? parseFloat(summary.market_value) : 0;
  const profitLoss = summary ? parseFloat(summary.profit_loss) : 0;
  const profitLossPct = summary ? parseFloat(summary.profit_loss_pct) : 0;
  const positionsCount = summary?.positions?.length || 0;

  const displayPrimaryValue = valuationMode === "market" ? marketValue : totalCost;
  const displayPrimaryLabel = valuationMode === "market" ? "Patrimônio a Mercado" : "Custo Total de Aquisição";

  const chartData = history.map((h) => ({
    date: h.date,
    patrimonio: parseFloat(h.market_value),
    custo: parseFloat(h.total_cost),
    cdi: h.cdi_rate ? parseFloat(h.cdi_rate) : undefined,
    selic: h.selic_rate ? parseFloat(h.selic_rate) : undefined,
    ipca: h.ipca_rate ? parseFloat(h.ipca_rate) : undefined,
  }));

  // If no history exists but summary has positions, provide a single current data point
  if (chartData.length === 0 && (marketValue > 0 || totalCost > 0)) {
    chartData.push({
      date: "Atual",
      patrimonio: marketValue,
      custo: totalCost,
      cdi: undefined,
      selic: undefined,
      ipca: undefined,
    });
  }

  return (
    <div className="flex min-h-screen flex-col bg-background">
      <Header valuationMode={valuationMode} onValuationModeChange={setValuationMode} lastUpdated={summary?.updated_at} />

      <main className="flex-1 p-4 sm:p-6 md:p-8">
        <div className="mx-auto max-w-7xl space-y-6">
          {/* Top Title & Refresh */}
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold tracking-tight text-foreground">Visão Geral</h1>
              <p className="text-xs sm:text-sm text-muted-foreground mt-0.5">
                Acompanhamento consolidado de custódia, cotações e rendimento fiscal
              </p>
            </div>
            <div className="flex items-center space-x-2">
              <Button
                variant="outline"
                size="sm"
                onClick={loadData}
                disabled={loading}
                className="text-xs h-8"
              >
                <RefreshCw className={`h-3.5 w-3.5 mr-1.5 ${loading ? "animate-spin" : ""}`} />
                Atualizar
              </Button>
            </div>
          </div>

          {error && (
            <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-xs text-destructive">
              {error}. Verifique se a API backend está em execução.
            </div>
          )}

          {/* 4 KPI Cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {/* KPI 1: Primary Value (Market or Cost based on toggle) */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <CardTitle className="text-xs font-medium text-muted-foreground">
                  {displayPrimaryLabel}
                </CardTitle>
                <DollarSign className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-xl sm:text-2xl font-bold tracking-tight">
                  {loading ? "..." : formatCurrency(displayPrimaryValue)}
                </div>
                <p className="text-[11px] text-muted-foreground mt-1">
                  {valuationMode === "market"
                    ? `Custo total: ${formatCurrency(totalCost)}`
                    : `Valor a mercado: ${formatCurrency(marketValue)}`}
                </p>
              </CardContent>
            </Card>

            {/* KPI 2: Profit / Loss */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <CardTitle className="text-xs font-medium text-muted-foreground">
                  Resultado Não Realizado
                </CardTitle>
                {profitLoss >= 0 ? (
                  <TrendingUp className="h-4 w-4 text-emerald-500" />
                ) : (
                  <TrendingDown className="h-4 w-4 text-destructive" />
                )}
              </CardHeader>
              <CardContent>
                <div
                  className={`text-xl sm:text-2xl font-bold tracking-tight ${
                    profitLoss >= 0 ? "text-emerald-600 dark:text-emerald-400" : "text-destructive"
                  }`}
                >
                  {loading ? "..." : formatCurrency(profitLoss)}
                </div>
                <p className="text-[11px] text-muted-foreground mt-1 flex items-center space-x-1">
                  <span>Rentabilidade:</span>
                  <span
                    className={`font-medium ${
                      profitLoss >= 0 ? "text-emerald-600 dark:text-emerald-400" : "text-destructive"
                    }`}
                  >
                    {formatPercent(profitLossPct)}
                  </span>
                </p>
              </CardContent>
            </Card>

            {/* KPI 3: Custody Assets */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <CardTitle className="text-xs font-medium text-muted-foreground">
                  Ativos em Custódia
                </CardTitle>
                <Layers className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-xl sm:text-2xl font-bold tracking-tight">
                  {loading ? "..." : positionsCount}
                </div>
                <p className="text-[11px] text-muted-foreground mt-1">
                  {positionsCount === 1 ? "1 ticker ativo" : `${positionsCount} tickers ativos`}
                </p>
              </CardContent>
            </Card>

            {/* KPI 4: Allocation Summary */}
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
                <CardTitle className="text-xs font-medium text-muted-foreground">
                  Alocação Principal
                </CardTitle>
                <PieChart className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-xl sm:text-2xl font-bold tracking-tight truncate">
                  {loading || !summary?.allocations?.[0]
                    ? "Diversificado"
                    : `${summary.allocations[0].asset_class} (${summary.allocations[0].weight_pct}%)`}
                </div>
                <p className="text-[11px] text-muted-foreground mt-1">
                  {summary?.allocations?.length || 0} classes de ativos
                </p>
              </CardContent>
            </Card>
          </div>

          {/* Chart & Allocation Grid */}
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Chart: 2 columns */}
            <Card className="lg:col-span-2">
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-semibold">Evolução Patrimonial</CardTitle>
                <CardDescription className="text-xs">
                  Comparação entre o valor de mercado atualizado e o custo ponderado de aquisição (PM)
                </CardDescription>
              </CardHeader>
              <CardContent className="pt-2">
                <PerformanceChart data={chartData} />
              </CardContent>
            </Card>

            {/* Allocation breakdown: 1 column */}
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-base font-semibold">Alocação por Classe</CardTitle>
                <CardDescription className="text-xs">
                  Distribuição percentual do patrimônio
                </CardDescription>
              </CardHeader>
              <CardContent className="pt-2">
                {summary?.allocations && summary.allocations.length > 0 ? (
                  <div className="flex flex-col items-center">
                    <div className="h-48 w-48">
                      <ResponsiveContainer width="100%" height="100%">
                        <RechartsPieChart>
                          <Pie
                            data={summary.allocations.map((a) => ({
                              name: a.asset_class,
                              value: parseFloat(a.market_value),
                            }))}
                            cx="50%"
                            cy="50%"
                            innerRadius={50}
                            outerRadius={75}
                            paddingAngle={2}
                            dataKey="value"
                            stroke="none"
                          >
                            {summary.allocations.map((_, idx) => (
                              <Cell
                                key={idx}
                                fill={DONUT_COLORS[idx % DONUT_COLORS.length]}
                              />
                            ))}
                          </Pie>
                          <Tooltip
                            formatter={(value: number) => formatCurrency(value)}
                            contentStyle={{
                              backgroundColor: "var(--card)",
                              border: "1px solid var(--border)",
                              borderRadius: "8px",
                              fontSize: "11px",
                            }}
                          />
                        </RechartsPieChart>
                      </ResponsiveContainer>
                    </div>
                    <div className="mt-3 space-y-1.5 w-full">
                      {summary.allocations.map((alloc, idx) => (
                        <div key={alloc.asset_class} className="flex items-center justify-between text-xs">
                          <span className="flex items-center space-x-1.5">
                            <span
                              className="inline-block h-2.5 w-2.5 rounded-full"
                              style={{ backgroundColor: DONUT_COLORS[idx % DONUT_COLORS.length] }}
                            />
                            <span className="font-medium text-foreground">{alloc.asset_class}</span>
                          </span>
                          <span className="text-muted-foreground">{alloc.weight_pct}%</span>
                        </div>
                      ))}
                    </div>
                  </div>
                ) : (
                  <div className="flex h-56 items-center justify-center text-xs text-muted-foreground border border-dashed border-border rounded-lg">
                    Nenhuma alocação registrada
                  </div>
                )}
              </CardContent>
            </Card>
          </div>

          {/* Consolidated Positions Table */}
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base font-semibold">Posição Consolidada</CardTitle>
              <CardDescription className="text-xs">
                Detalhamento por ativo em custódia com preço médio ponderado e resultado não realizado
              </CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="border-b border-border bg-secondary/30 text-muted-foreground font-medium">
                    <tr>
                      <th className="py-3 px-4">Ativo</th>
                      <th className="py-3 px-4">Classe</th>
                      <th className="py-3 px-4 text-right">Qtd</th>
                      <th className="py-3 px-4 text-right">Preço Médio</th>
                      <th className="py-3 px-4 text-right">Cotação Atual</th>
                      <th className="py-3 px-4 text-right">Custo Total</th>
                      <th className="py-3 px-4 text-right">Valor Mercado</th>
                      <th className="py-3 px-4 text-right">Resultado</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {summary?.positions && summary.positions.length > 0 ? (
                      summary.positions.map((pos) => {
                        const pl = parseFloat(pos.profit_loss);
                        const plPct = parseFloat(pos.profit_loss_pct);
                        return (
                          <tr key={pos.asset_id} className="hover:bg-muted/30 transition-colors">
                            <td className="py-3 px-4 font-semibold text-foreground">
                              <div>{pos.ticker}</div>
                              <div className="text-[10px] text-muted-foreground font-normal truncate max-w-[140px]">
                                {pos.name}
                              </div>
                            </td>
                            <td className="py-3 px-4">
                              <Badge variant="secondary" className="text-[10px]">
                                {pos.asset_class}
                              </Badge>
                            </td>
                            <td className="py-3 px-4 text-right font-mono">
                              {pos.quantity}
                            </td>
                            <td className="py-3 px-4 text-right font-mono text-muted-foreground">
                              {formatCurrency(pos.average_price)}
                            </td>
                            <td className="py-3 px-4 text-right font-mono font-medium">
                              {formatCurrency(pos.current_price)}
                            </td>
                            <td className="py-3 px-4 text-right font-mono text-muted-foreground">
                              {formatCurrency(pos.total_cost)}
                            </td>
                            <td className="py-3 px-4 text-right font-mono font-semibold">
                              {formatCurrency(pos.market_value)}
                            </td>
                            <td className="py-3 px-4 text-right font-mono">
                              <span
                                className={`font-semibold ${
                                  pl >= 0
                                    ? "text-emerald-600 dark:text-emerald-400"
                                    : "text-destructive"
                                }`}
                              >
                                {formatCurrency(pl)}
                              </span>
                              <div
                                className={`text-[10px] ${
                                  pl >= 0
                                    ? "text-emerald-600 dark:text-emerald-400"
                                    : "text-destructive"
                                }`}
                              >
                                {formatPercent(plPct)}
                              </div>
                            </td>
                          </tr>
                        );
                      })
                    ) : (
                      <tr>
                        <td colSpan={8} className="py-8 text-center text-muted-foreground">
                          Nenhum ativo em custódia no momento. Importe uma nota de corretagem em PDF para começar.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>
        </div>
      </main>
    </div>
  );
}
