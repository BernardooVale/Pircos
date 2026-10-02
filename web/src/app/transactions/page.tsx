"use client";

import { useEffect, useState } from "react";
import { Header } from "@/components/header";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { fetchTransactions } from "@/lib/api";
import { formatCurrency, formatDate } from "@/lib/utils";
import { RefreshCw, Layers } from "lucide-react";

export default function TransactionsPage() {
  const [transactions, setTransactions] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  async function loadData() {
    try {
      setLoading(true);
      const data = await fetchTransactions();
      setTransactions(data || []);
    } catch {
      setTransactions([]);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadData();
  }, []);

  return (
    <div className="flex min-h-screen flex-col bg-background">
      <Header />

      <main className="flex-1 p-4 sm:p-6 md:p-8">
        <div className="mx-auto max-w-7xl space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold tracking-tight text-foreground">Livro de Transações</h1>
              <p className="text-xs sm:text-sm text-muted-foreground mt-0.5">
                Histórico contábil completo de todas as compras, vendas e proventos registrados
              </p>
            </div>
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

          <Card>
            <CardHeader className="py-3 px-4 border-b border-border">
              <div className="flex items-center justify-between">
                <CardTitle className="text-base font-semibold">Transações ({transactions.length})</CardTitle>
                <Badge variant="outline" className="text-[10px]">
                  Ordem cronológica
                </Badge>
              </div>
            </CardHeader>
            <CardContent className="p-0">
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs">
                  <thead className="bg-secondary/40 text-muted-foreground font-medium border-b border-border">
                    <tr>
                      <th className="py-2.5 px-4">Data</th>
                      <th className="py-2.5 px-4">Tipo</th>
                      <th className="py-2.5 px-4">Ativo</th>
                      <th className="py-2.5 px-4 text-right">Quantidade</th>
                      <th className="py-2.5 px-4 text-right">Preço Unit.</th>
                      <th className="py-2.5 px-4 text-right">Custos</th>
                      <th className="py-2.5 px-4 text-right">Total</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {transactions.length > 0 ? (
                      transactions.map((tx: any) => (
                        <tr key={tx.id} className="hover:bg-muted/20 transition-colors">
                          <td className="py-2.5 px-4 font-mono text-[11px] text-muted-foreground">
                            {formatDate(tx.operation_date)}
                          </td>
                          <td className="py-2.5 px-4">
                            <span
                              className={`text-[10px] font-bold px-2 py-0.5 rounded ${
                                tx.operation_type === "BUY"
                                  ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                                  : "bg-destructive/10 text-destructive"
                              }`}
                            >
                              {tx.operation_type === "BUY" ? "COMPRA" : "VENDA"}
                            </span>
                          </td>
                          <td className="py-2.5 px-4 font-semibold text-foreground">
                            <div>{tx.ticker || tx.asset_id}</div>
                            {tx.asset_name && (
                              <div className="text-[10px] text-muted-foreground font-normal truncate max-w-[150px]">
                                {tx.asset_name}
                              </div>
                            )}
                          </td>
                          <td className="py-2.5 px-4 text-right font-mono">
                            {tx.quantity}
                          </td>
                          <td className="py-2.5 px-4 text-right font-mono text-muted-foreground">
                            {formatCurrency(tx.unit_price)}
                          </td>
                          <td className="py-2.5 px-4 text-right font-mono text-muted-foreground">
                            {formatCurrency(tx.costs)}
                          </td>
                          <td className="py-2.5 px-4 text-right font-mono font-semibold">
                            {formatCurrency(tx.total_amount)}
                          </td>
                        </tr>
                      ))
                    ) : (
                      <tr>
                        <td colSpan={7} className="py-12 text-center text-muted-foreground">
                          Nenhuma transação encontrada.
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
