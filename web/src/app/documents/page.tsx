"use client";

import { useState } from "react";
import { Header } from "@/components/header";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { parseBrokerageNote, confirmBrokerageNote, ParsedBrokerageNote, ParsedTransaction } from "@/lib/api";
import { formatCurrency } from "@/lib/utils";
import { UploadCloud, FileText, CheckCircle2, AlertCircle, Plus, Trash2, ArrowRight } from "lucide-react";
import Link from "next/link";

export default function DocumentsPage() {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [pdfUrl, setPdfUrl] = useState<string | null>(null);
  const [parsedData, setParsedData] = useState<ParsedBrokerageNote | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // Form editable state
  const [noteNumber, setNoteNumber] = useState("");
  const [brokerName, setBrokerName] = useState("");
  const [operationDate, setOperationDate] = useState("");
  const [settlementDate, setSettlementDate] = useState("");
  const [totalCosts, setTotalCosts] = useState("0.00");
  const [transactions, setTransactions] = useState<ParsedTransaction[]>([]);

  function handleFileSelect(file: File) {
    setSelectedFile(file);
    const url = URL.createObjectURL(file);
    setPdfUrl(url);
    setError(null);
    setSuccess(null);
    parseFile(file);
  }

  async function parseFile(file: File) {
    try {
      setLoading(true);
      setError(null);
      const data = await parseBrokerageNote(file);
      setParsedData(data);

      // Populate form
      setNoteNumber(data.note_number || "");
      setBrokerName(data.broker_name || "");
      if (data.operation_date) {
        setOperationDate(data.operation_date.split("T")[0]);
      }
      if (data.settlement_date) {
        setSettlementDate(data.settlement_date.split("T")[0]);
      }
      setTotalCosts(data.total_costs || "0.00");
      setTransactions(data.transactions || []);
    } catch (err: any) {
      setError(err?.message || "Falha ao processar a nota de corretagem em PDF");
    } finally {
      setLoading(false);
    }
  }

  function handleUpdateTransaction(index: number, field: keyof ParsedTransaction, value: any) {
    setTransactions((prev) => {
      const updated = [...prev];
      updated[index] = { ...updated[index], [field]: value };

      // Recalculate total amount if quantity or price changed
      if (field === "quantity" || field === "unit_price") {
        const qty = parseFloat(updated[index].quantity) || 0;
        const price = parseFloat(updated[index].unit_price) || 0;
        updated[index].total_amount = (qty * price).toFixed(2);
      }
      return updated;
    });
  }

  function handleRemoveTransaction(index: number) {
    setTransactions((prev) => prev.filter((_, i) => i !== index));
  }

  function handleAddTransaction() {
    setTransactions((prev) => [
      ...prev,
      {
        operation_type: "BUY",
        ticker: "NOVO3",
        asset_description: "ATIVO MANUAL",
        asset_class: "STOCKS",
        quantity: "100",
        unit_price: "10.00",
        total_amount: "1000.00",
        costs: "0.00",
        is_day_trade: false,
      },
    ]);
  }

  async function handleConfirm() {
    if (!selectedFile) return;

    try {
      setSaving(true);
      setError(null);

      const payload = {
        note_number: noteNumber,
        broker_name: brokerName,
        operation_date: operationDate,
        settlement_date: settlementDate,
        total_costs: totalCosts,
        transactions: transactions.map((t) => ({
          operation_type: t.operation_type,
          ticker: t.ticker.toUpperCase(),
          asset_description: t.asset_description,
          asset_class: t.asset_class,
          quantity: t.quantity,
          unit_price: t.unit_price,
          total_amount: t.total_amount,
          costs: t.costs,
          is_day_trade: t.is_day_trade,
        })),
      };

      const res = await confirmBrokerageNote(payload, selectedFile);
      setSuccess(`Nota nº ${noteNumber || "registrada"} salva com sucesso! ${res.transactions_saved} transações gravadas.`);
      setParsedData(null);
    } catch (err: any) {
      setError(err?.message || "Erro ao salvar e confirmar a nota");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex min-h-screen flex-col bg-background">
      <Header />

      <main className="flex-1 p-4 sm:p-6 md:p-8">
        <div className="mx-auto max-w-7xl space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold tracking-tight text-foreground">Ingestão de Notas de Corretagem</h1>
              <p className="text-xs sm:text-sm text-muted-foreground mt-0.5">
                Faça upload de PDFs padrão Sinacor (B3), confira as operações e confirme com recálculo automático
              </p>
            </div>
            {selectedFile && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setSelectedFile(null);
                  setPdfUrl(null);
                  setParsedData(null);
                  setSuccess(null);
                }}
                className="text-xs"
              >
                Nova Nota
              </Button>
            )}
          </div>

          {/* Feedback banners */}
          {error && (
            <div className="flex items-center space-x-2 rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-xs text-destructive">
              <AlertCircle className="h-4 w-4 shrink-0" />
              <span>{error}</span>
            </div>
          )}
          {success && (
            <div className="flex items-center justify-between rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-3 text-xs text-emerald-600 dark:text-emerald-400">
              <div className="flex items-center space-x-2">
                <CheckCircle2 className="h-4 w-4 shrink-0" />
                <span>{success}</span>
              </div>
              <Link href="/" className="inline-flex items-center space-x-1 font-semibold underline underline-offset-4">
                <span>Ver no Dashboard</span>
                <ArrowRight className="h-3 w-3" />
              </Link>
            </div>
          )}

          {/* If no file selected: Upload Area */}
          {!selectedFile && (
            <Card className="border-dashed border-2">
              <CardContent className="flex flex-col items-center justify-center py-16 px-4 text-center">
                <div className="rounded-full bg-secondary p-4 mb-4">
                  <UploadCloud className="h-8 w-8 text-muted-foreground" />
                </div>
                <h3 className="text-base font-semibold text-foreground">
                  Selecione ou arraste sua nota de corretagem (PDF)
                </h3>
                <p className="text-xs text-muted-foreground mt-1 max-w-md">
                  Compatível com notas de corretagem da B3 no padrão Sinacor (XP, Clear, Rico, NuInvest, BTG, Inter, etc.)
                </p>
                <div className="mt-6">
                  <input
                    id="pdf-upload"
                    type="file"
                    accept="application/pdf"
                    className="hidden"
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file) handleFileSelect(file);
                    }}
                  />
                  <Button
                    type="button"
                    variant="default"
                    size="sm"
                    className="cursor-pointer text-xs"
                    onClick={() => document.getElementById("pdf-upload")?.click()}
                  >
                    Selecionar Arquivo PDF
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Split Screen View */}
          {selectedFile && (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 items-start">
              {/* Left Side: Native PDF Viewer */}
              <Card className="h-[750px] flex flex-col overflow-hidden">
                <CardHeader className="py-3 px-4 border-b border-border bg-secondary/30">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-2 truncate">
                      <FileText className="h-4 w-4 text-muted-foreground shrink-0" />
                      <span className="text-xs font-semibold truncate">{selectedFile.name}</span>
                    </div>
                    <Badge variant="outline" className="text-[10px]">
                      {(selectedFile.size / 1024).toFixed(1)} KB
                    </Badge>
                  </div>
                </CardHeader>
                <CardContent className="p-0 flex-1 bg-muted/20">
                  {pdfUrl ? (
                    <iframe
                      src={pdfUrl}
                      className="w-full h-full border-none"
                      title="Visualizador de PDF"
                    />
                  ) : (
                    <div className="flex h-full items-center justify-center text-xs text-muted-foreground">
                      Carregando documento...
                    </div>
                  )}
                </CardContent>
              </Card>

              {/* Right Side: Verification and Editing Form */}
              <Card className="flex flex-col">
                <CardHeader className="py-3 px-4 border-b border-border">
                  <CardTitle className="text-base font-semibold">Conferência dos Dados Extraídos</CardTitle>
                  <CardDescription className="text-xs">
                    Revise os campos detectados pelo parser antes de confirmar a gravação
                  </CardDescription>
                </CardHeader>
                <CardContent className="p-4 space-y-4">
                  {loading ? (
                    <div className="flex h-64 items-center justify-center text-xs text-muted-foreground">
                      Analisando PDF estruturado com regex Sinacor...
                    </div>
                  ) : (
                    <>
                      {/* Note Header Metadata */}
                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 p-3 rounded-lg bg-secondary/30 border border-border">
                        <div>
                          <label className="text-[10px] font-medium text-muted-foreground">Nr. Nota</label>
                          <Input
                            value={noteNumber}
                            onChange={(e) => setNoteNumber(e.target.value)}
                            className="h-7 text-xs mt-1"
                          />
                        </div>
                        <div>
                          <label className="text-[10px] font-medium text-muted-foreground">Corretora</label>
                          <Input
                            value={brokerName}
                            onChange={(e) => setBrokerName(e.target.value)}
                            className="h-7 text-xs mt-1"
                          />
                        </div>
                        <div>
                          <label className="text-[10px] font-medium text-muted-foreground">Data Pregão</label>
                          <Input
                            type="date"
                            value={operationDate}
                            onChange={(e) => setOperationDate(e.target.value)}
                            className="h-7 text-xs mt-1"
                          />
                        </div>
                        <div>
                          <label className="text-[10px] font-medium text-muted-foreground">Total Custos (R$)</label>
                          <Input
                            value={totalCosts}
                            onChange={(e) => setTotalCosts(e.target.value)}
                            className="h-7 text-xs mt-1"
                          />
                        </div>
                      </div>

                      {/* Operations Table */}
                      <div className="space-y-2">
                        <div className="flex items-center justify-between">
                          <h4 className="text-xs font-semibold text-foreground">
                            Operações Detectadas ({transactions.length})
                          </h4>
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={handleAddTransaction}
                            className="h-7 text-[11px] px-2"
                          >
                            <Plus className="h-3 w-3 mr-1" />
                            Adicionar Linha
                          </Button>
                        </div>

                        <div className="space-y-2 max-h-[380px] overflow-y-auto pr-1">
                          {transactions.map((tx, idx) => (
                            <div
                              key={idx}
                              className="rounded-lg border border-border bg-card p-3 space-y-2 text-xs"
                            >
                              <div className="flex items-center justify-between">
                                <div className="flex items-center space-x-2">
                                  {/* Buy / Sell toggle */}
                                  <button
                                    type="button"
                                    onClick={() =>
                                      handleUpdateTransaction(
                                        idx,
                                        "operation_type",
                                        tx.operation_type === "BUY" ? "SELL" : "BUY"
                                      )
                                    }
                                    className={`px-2 py-0.5 rounded text-[10px] font-bold ${
                                      tx.operation_type === "BUY"
                                        ? "bg-emerald-500/20 text-emerald-600 dark:text-emerald-400"
                                        : "bg-destructive/20 text-destructive"
                                    }`}
                                  >
                                    {tx.operation_type === "BUY" ? "COMPRA" : "VENDA"}
                                  </button>

                                  <input
                                    type="text"
                                    value={tx.ticker}
                                    onChange={(e) => handleUpdateTransaction(idx, "ticker", e.target.value)}
                                    className="w-20 font-bold uppercase rounded border border-input bg-background px-1.5 py-0.5 text-xs text-foreground"
                                    placeholder="TICKER"
                                  />

                                  <select
                                    value={tx.asset_class}
                                    onChange={(e) => handleUpdateTransaction(idx, "asset_class", e.target.value)}
                                    className="rounded border border-input bg-background px-1.5 py-0.5 text-[11px] text-muted-foreground"
                                  >
                                    <option value="STOCKS">Ações</option>
                                    <option value="FII">FII / FIAGRO</option>
                                    <option value="ETF">ETF</option>
                                    <option value="BDR">BDR</option>
                                  </select>
                                </div>

                                <Button
                                  type="button"
                                  variant="ghost"
                                  size="icon"
                                  onClick={() => handleRemoveTransaction(idx)}
                                  className="h-6 w-6 text-muted-foreground hover:text-destructive"
                                >
                                  <Trash2 className="h-3.5 w-3.5" />
                                </Button>
                              </div>

                              {/* Numbers Row */}
                              <div className="grid grid-cols-4 gap-2 text-[11px]">
                                <div>
                                  <label className="text-[10px] text-muted-foreground">Qtd</label>
                                  <Input
                                    value={tx.quantity}
                                    onChange={(e) => handleUpdateTransaction(idx, "quantity", e.target.value)}
                                    className="h-7 text-xs font-mono"
                                  />
                                </div>
                                <div>
                                  <label className="text-[10px] text-muted-foreground">Preço (R$)</label>
                                  <Input
                                    value={tx.unit_price}
                                    onChange={(e) => handleUpdateTransaction(idx, "unit_price", e.target.value)}
                                    className="h-7 text-xs font-mono"
                                  />
                                </div>
                                <div>
                                  <label className="text-[10px] text-muted-foreground">Custos (R$)</label>
                                  <Input
                                    value={tx.costs}
                                    onChange={(e) => handleUpdateTransaction(idx, "costs", e.target.value)}
                                    className="h-7 text-xs font-mono text-muted-foreground"
                                  />
                                </div>
                                <div>
                                  <label className="text-[10px] text-muted-foreground">Total Líq (R$)</label>
                                  <div className="h-7 flex items-center font-mono font-semibold text-foreground">
                                    {formatCurrency(tx.total_amount)}
                                  </div>
                                </div>
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>

                      {/* Confirm and Save button */}
                      <div className="pt-2 border-t border-border flex items-center justify-between">
                        <div className="text-xs text-muted-foreground">
                          {transactions.length} operação(ões) prontas para gravação
                        </div>
                        <Button
                          variant="default"
                          size="sm"
                          onClick={handleConfirm}
                          disabled={saving || transactions.length === 0}
                          className="text-xs font-semibold px-4"
                        >
                          {saving ? "Gravando e recalculando..." : "Confirmar e Gravar Nota"}
                        </Button>
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
