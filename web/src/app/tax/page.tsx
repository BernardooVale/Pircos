"use client";

import { useEffect, useState } from "react";
import { Header } from "@/components/header";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { fetchMonthlyTax, fetchTaxDrilldown, MonthCard, MonthlyPreviewResponse } from "@/lib/api";
import { formatCurrency, formatPercent, formatDate } from "@/lib/utils";
import { Calendar, DollarSign, ArrowUpRight, TrendingDown, TrendingUp, X, ChevronRight, RefreshCw, FileText } from "lucide-react";

const MONTH_NAMES = [
  "Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho",
  "Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro",
];

export default function TaxPage() {
  const currentYear = new Date().getFullYear().toString();
  const [selectedYear, setSelectedYear] = useState<string>("2024");
  const [taxData, setTaxData] = useState<MonthlyPreviewResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Drilldown modal state
  const [drilldownMonth, setDrilldownMonth] = useState<MonthCard | null>(null);
  const [drilldownData, setDrilldownData] = useState<any | null>(null);
  const [drilldownLoading, setDrilldownLoading] = useState(false);
  const [selectedBucket, setSelectedBucket] = useState<string>("STOCKS_SWING");

  async function loadTax(year: string) {
    try {
      setLoading(true);
      setError(null);
      const data = await fetchMonthlyTax(year);
      setTaxData(data);
    } catch (err: any) {
      setError(err?.message || "Erro ao carregar prévia de IR");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadTax(selectedYear);
  }, [selectedYear]);

  async function openDrilldown(month: MonthCard) {
    setDrilldownMonth(month);
    const firstBucket = month.buckets?.[0]?.asset_bucket || "STOCKS_SWING";
    setSelectedBucket(firstBucket);
    loadDrilldown(month.year_month, firstBucket);
  }

  async function loadDrilldown(yearMonth: string, bucket: string) {
    try {
      setDrilldownLoading(true);
      const data = await fetchTaxDrilldown(yearMonth, bucket);
      setDrilldownData(data);
    } catch {
      setDrilldownData(null);
    } finally {
      setDrilldownLoading(false);
    }
  }

  function handleBucketChange(bucket: string) {
    setSelectedBucket(bucket);
    if (drilldownMonth) {
      loadDrilldown(drilldownMonth.year_month, bucket);
    }
  }

  return (
    <div className="flex min-h-screen flex-col bg-background">
      <Header />

      <main className="flex-1 p-4 sm:p-6 md:p-8">
        <div className="mx-auto max-w-7xl space-y-6">
          {/* Header & Year Selector */}
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold tracking-tight text-foreground">Prévia de Imposto de Renda</h1>
              <p className="text-xs sm:text-sm text-muted-foreground mt-0.5">
                Cálculo de DARFs mensais, regras de isenção de R$ 20k em ações e compensação perpétua de prejuízos
              </p>
            </div>

            {/* Year Selector */}
            <div className="flex items-center space-x-2">
              <div className="flex rounded-lg border border-border bg-secondary/50 p-0.5 text-xs">
                {["2023", "2024", "2025", "2026"].map((y) => (
                  <button
                    key={y}
                    type="button"
                    onClick={() => setSelectedYear(y)}
                    className={`px-3 py-1 rounded-md font-medium transition-all ${
                      selectedYear === y
                        ? "bg-card text-foreground shadow-sm font-semibold"
                        : "text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {y}
                  </button>
                ))}
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => loadTax(selectedYear)}
                disabled={loading}
                className="h-8 text-xs"
              >
                <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
              </Button>
            </div>
          </div>

          {error && (
            <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-xs text-destructive">
              {error}
            </div>
          )}

          {/* Annual Summary KPI Banner */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-xs font-medium text-muted-foreground">Ano Calendário</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{selectedYear}</div>
                <p className="text-[11px] text-muted-foreground mt-1">Declaração Anual IRPF</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-xs font-medium text-muted-foreground">Total de Imposto Apurado</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold font-mono">
                  {loading ? "..." : formatCurrency(taxData?.total_tax_due || "0")}
                </div>
                <p className="text-[11px] text-muted-foreground mt-1">Imposto bruto devido no ano</p>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-xs font-medium text-muted-foreground">Total de DARFs no Ano</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400">
                  {loading ? "..." : formatCurrency(taxData?.total_darf || "0")}
                </div>
                <p className="text-[11px] text-muted-foreground mt-1">Líquido a pagar após retenção de IRRF</p>
              </CardContent>
            </Card>
          </div>

          {/* 12 Monthly Cards Grid */}
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
            {taxData?.months ? (
              taxData.months.map((card) => {
                const monthName = MONTH_NAMES[card.month_number - 1];
                const darfVal = parseFloat(card.final_darf);
                const salesVal = parseFloat(card.total_sales);

                let badgeVariant: "default" | "secondary" | "destructive" | "outline" | "success" = "secondary";
                let badgeText = "SEM MOVIMENTO";

                if (darfVal >= 10) {
                  badgeVariant = "default";
                  badgeText = "A PAGAR";
                } else if (salesVal > 0 && darfVal === 0) {
                  badgeVariant = "success";
                  badgeText = "ISENTO";
                }

                return (
                  <Card
                    key={card.year_month}
                    className="hover:border-foreground/30 transition-all cursor-pointer group flex flex-col justify-between"
                    onClick={() => openDrilldown(card)}
                  >
                    <CardHeader className="pb-2">
                      <div className="flex items-center justify-between">
                        <span className="font-semibold text-sm text-foreground">{monthName}</span>
                        <Badge variant={badgeVariant} className="text-[10px] px-2 py-0">
                          {badgeText}
                        </Badge>
                      </div>
                      <CardDescription className="text-[10px] font-mono">
                        {card.year_month}
                      </CardDescription>
                    </CardHeader>

                    <CardContent className="space-y-2 pt-0 pb-4">
                      <div className="space-y-1 text-xs">
                        <div className="flex justify-between text-muted-foreground">
                          <span>Vendas:</span>
                          <span className="font-mono text-foreground font-medium">
                            {formatCurrency(card.total_sales)}
                          </span>
                        </div>
                        <div className="flex justify-between text-muted-foreground">
                          <span>Base Tributável:</span>
                          <span className="font-mono text-foreground font-medium">
                            {formatCurrency(card.taxable_base)}
                          </span>
                        </div>
                        <div className="flex justify-between text-muted-foreground pt-1 border-t border-border/60">
                          <span className="font-medium text-foreground">DARF:</span>
                          <span
                            className={`font-mono font-bold ${
                              darfVal >= 10 ? "text-emerald-600 dark:text-emerald-400" : "text-muted-foreground"
                            }`}
                          >
                            {formatCurrency(card.final_darf)}
                          </span>
                        </div>
                      </div>

                      {/* Buckets tags */}
                      {card.buckets && card.buckets.length > 0 && (
                        <div className="flex flex-wrap gap-1 pt-2">
                          {card.buckets.map((b) => (
                            <span
                              key={b.asset_bucket}
                              className="text-[9px] px-1.5 py-0.5 rounded bg-secondary text-secondary-foreground font-mono"
                            >
                              {b.asset_bucket}
                            </span>
                          ))}
                        </div>
                      )}
                    </CardContent>

                    <div className="px-6 py-2 bg-secondary/30 border-t border-border text-[11px] flex items-center justify-between text-muted-foreground group-hover:text-foreground transition-colors">
                      <span>Ver Raio-X</span>
                      <ChevronRight className="h-3.5 w-3.5" />
                    </div>
                  </Card>
                );
              })
            ) : (
              <div className="col-span-full py-12 text-center text-xs text-muted-foreground">
                Carregando calendário fiscal...
              </div>
            )}
          </div>
        </div>
      </main>

      {/* Drill-down Modal (Raio-X das Operações) */}
      {drilldownMonth && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 overflow-y-auto">
          <div className="relative w-full max-w-3xl rounded-xl border border-border bg-card shadow-2xl overflow-hidden my-8">
            {/* Modal Header */}
            <div className="flex items-center justify-between p-5 border-b border-border bg-secondary/20">
              <div>
                <h3 className="text-base font-bold text-foreground">
                  Raio-X Fiscal — {MONTH_NAMES[drilldownMonth.month_number - 1]} / {selectedYear}
                </h3>
                <p className="text-xs text-muted-foreground mt-0.5">
                  Detalhamento de alienações, base de cálculo e compensação de prejuízos
                </p>
              </div>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 rounded-full"
                onClick={() => setDrilldownMonth(null)}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>

            {/* Modal Content */}
            <div className="p-5 space-y-5">
              {/* Bucket Selector Tabs */}
              <div className="flex flex-wrap gap-1 border-b border-border pb-3">
                {["STOCKS_SWING", "STOCKS_DAYTRADE", "FII", "ETF", "CRYPTO"].map((b) => (
                  <button
                    key={b}
                    type="button"
                    onClick={() => handleBucketChange(b)}
                    className={`px-3 py-1 rounded-md text-xs font-medium transition-colors ${
                      selectedBucket === b
                        ? "bg-primary text-primary-foreground font-semibold shadow-sm"
                        : "bg-secondary text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {b}
                  </button>
                ))}
              </div>

              {drilldownLoading ? (
                <div className="flex h-48 items-center justify-center text-xs text-muted-foreground">
                  Carregando raio-X do balde...
                </div>
              ) : (
                <>
                  {/* Fiscal Balance Overview */}
                  {drilldownData?.balance ? (
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 p-3.5 rounded-lg bg-secondary/30 border border-border text-xs">
                      <div>
                        <div className="text-[10px] text-muted-foreground">Volume Alienado</div>
                        <div className="font-mono font-semibold mt-0.5">
                          {formatCurrency(drilldownData.balance.total_sales)}
                        </div>
                      </div>
                      <div>
                        <div className="text-[10px] text-muted-foreground">Lucro Bruto Apurado</div>
                        <div className="font-mono font-semibold text-emerald-600 dark:text-emerald-400 mt-0.5">
                          {formatCurrency(drilldownData.balance.gross_profit)}
                        </div>
                      </div>
                      <div>
                        <div className="text-[10px] text-muted-foreground">Prejuízo Compensado</div>
                        <div className="font-mono font-semibold text-destructive mt-0.5">
                          {formatCurrency(drilldownData.balance.losses_deducted)}
                        </div>
                      </div>
                      <div>
                        <div className="text-[10px] text-muted-foreground">Prejuízo p/ Meses Seguintes</div>
                        <div className="font-mono font-semibold text-muted-foreground mt-0.5">
                          {formatCurrency(drilldownData.balance.accumulated_loss_carried)}
                        </div>
                      </div>
                    </div>
                  ) : (
                    <div className="p-3 text-center text-xs text-muted-foreground bg-secondary/20 rounded-lg">
                      Nenhuma atividade registrada para a categoria {selectedBucket} neste mês.
                    </div>
                  )}

                  {/* Individual Sales List */}
                  <div className="space-y-2">
                    <h4 className="text-xs font-semibold text-foreground flex items-center space-x-1.5">
                      <FileText className="h-3.5 w-3.5" />
                      <span>Alienações do Mês ({drilldownData?.sales?.length || 0})</span>
                    </h4>

                    {drilldownData?.sales && drilldownData.sales.length > 0 ? (
                      <div className="overflow-x-auto rounded-lg border border-border">
                        <table className="w-full text-left text-xs">
                          <thead className="bg-secondary/40 text-muted-foreground font-medium border-b border-border">
                            <tr>
                              <th className="py-2.5 px-3">Data</th>
                              <th className="py-2.5 px-3">Ativo</th>
                              <th className="py-2.5 px-3 text-right">Qtd</th>
                              <th className="py-2.5 px-3 text-right">Preço Venda</th>
                              <th className="py-2.5 px-3 text-right">Volume</th>
                              <th className="py-2.5 px-3 text-right">Resultado</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-border">
                            {drilldownData.sales.map((sale: any, idx: number) => {
                              const pl = parseFloat(sale.profit_loss);
                              return (
                                <tr key={idx} className="hover:bg-muted/20">
                                  <td className="py-2 px-3 text-muted-foreground font-mono text-[11px]">
                                    {formatDate(sale.operation_date)}
                                  </td>
                                  <td className="py-2 px-3 font-semibold text-foreground">
                                    {sale.ticker}
                                  </td>
                                  <td className="py-2 px-3 text-right font-mono">
                                    {sale.quantity}
                                  </td>
                                  <td className="py-2 px-3 text-right font-mono text-muted-foreground">
                                    {formatCurrency(sale.unit_price)}
                                  </td>
                                  <td className="py-2 px-3 text-right font-mono font-medium">
                                    {formatCurrency(sale.sale_amount)}
                                  </td>
                                  <td className="py-2 px-3 text-right font-mono font-semibold">
                                    <span className={pl >= 0 ? "text-emerald-600 dark:text-emerald-400" : "text-destructive"}>
                                      {formatCurrency(pl)}
                                    </span>
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    ) : (
                      <div className="py-6 text-center text-xs text-muted-foreground border border-dashed border-border rounded-lg">
                        Nenhuma venda registrada para este balde neste mês.
                      </div>
                    )}
                  </div>
                </>
              )}
            </div>

            {/* Modal Footer */}
            <div className="p-4 border-t border-border bg-secondary/10 flex justify-end">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setDrilldownMonth(null)}
                className="text-xs"
              >
                Fechar
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
