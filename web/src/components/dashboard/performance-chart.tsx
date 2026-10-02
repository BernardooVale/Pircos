"use client";

import {
  ResponsiveContainer,
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Legend,
} from "recharts";
import { formatCurrency } from "@/lib/utils";

interface ChartDataPoint {
  date: string;
  patrimonio: number;
  custo: number;
  cdi?: number;
  selic?: number;
  ipca?: number;
}

interface PerformanceChartProps {
  data: ChartDataPoint[];
}

export function PerformanceChart({ data }: PerformanceChartProps) {
  if (!data || data.length === 0) {
    return (
      <div className="flex h-64 items-center justify-center rounded-xl border border-dashed border-border text-xs text-muted-foreground">
        Nenhum dado histórico registrado para exibir o gráfico
      </div>
    );
  }

  return (
    <div className="h-72 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <LineChart data={data} margin={{ top: 10, right: 10, left: 10, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="currentColor" className="text-border/60" vertical={false} />
          <XAxis
            dataKey="date"
            stroke="currentColor"
            className="text-muted-foreground text-[10px]"
            tickLine={false}
            axisLine={false}
          />
          <YAxis
            stroke="currentColor"
            className="text-muted-foreground text-[10px]"
            tickLine={false}
            axisLine={false}
            tickFormatter={(val) => `R$ ${(val / 1000).toFixed(0)}k`}
          />
          <Tooltip
            content={({ active, payload, label }) => {
              if (active && payload && payload.length) {
                return (
                  <div className="rounded-lg border border-border bg-card p-2.5 shadow-md text-xs space-y-1">
                    <p className="font-medium text-foreground">{label}</p>
                    {payload.map((entry, idx) => (
                      <div key={idx} className="flex items-center justify-between space-x-3">
                        <span className="flex items-center space-x-1.5 text-muted-foreground">
                          <span
                            className="inline-block h-2 w-2 rounded-full"
                            style={{ backgroundColor: entry.color }}
                          />
                          <span>{entry.name}:</span>
                        </span>
                        <span className="font-semibold text-foreground">
                          {["cdi", "selic", "ipca"].includes(entry.dataKey as string)
                            ? `${(entry.value as number).toFixed(2)}%`
                            : formatCurrency(entry.value as number)}
                        </span>
                      </div>
                    ))}
                  </div>
                );
              }
              return null;
            }}
          />
          <Legend
            verticalAlign="top"
            align="right"
            wrapperStyle={{ paddingBottom: 12, fontSize: 11 }}
          />
          <Line
            type="monotone"
            dataKey="patrimonio"
            name="Valor de Mercado"
            stroke="#10b981"
            strokeWidth={2}
            dot={false}
            activeDot={{ r: 4 }}
          />
          <Line
            type="monotone"
            dataKey="custo"
            name="Custo Total (PM)"
            stroke="#71717a"
            strokeWidth={1.5}
            strokeDasharray="4 4"
            dot={false}
            activeDot={{ r: 4 }}
          />
          <Line type="monotone" dataKey="cdi" name="CDI" stroke="#6366f1" strokeWidth={1} strokeDasharray="6 3" dot={false} />
          <Line type="monotone" dataKey="selic" name="Selic" stroke="#8b5cf6" strokeWidth={1} strokeDasharray="6 3" dot={false} />
          <Line type="monotone" dataKey="ipca" name="IPCA" stroke="#f59e0b" strokeWidth={1} strokeDasharray="6 3" dot={false} />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
