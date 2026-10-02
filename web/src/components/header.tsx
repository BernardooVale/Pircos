"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTheme } from "next-themes";
import { useEffect, useState } from "react";
import { Moon, Sun, DollarSign, TrendingUp, FileText, PieChart, Layers, Clock } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface HeaderProps {
  valuationMode?: "market" | "cost";
  onValuationModeChange?: (mode: "market" | "cost") => void;
  lastUpdated?: string;
}

export function Header({ valuationMode = "market", onValuationModeChange, lastUpdated }: HeaderProps) {
  const pathname = usePathname();
  const { theme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const navItems = [
    { href: "/", label: "Dashboard", icon: PieChart },
    { href: "/documents", label: "Notas (PDF)", icon: FileText },
    { href: "/tax", label: "Imposto de Renda", icon: DollarSign },
    { href: "/transactions", label: "Transações", icon: Layers },
  ];

  return (
    <header className="sticky top-0 z-50 w-full border-b border-border/60 bg-background/80 backdrop-blur-md">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4 sm:px-6">
        {/* Brand */}
        <div className="flex items-center space-x-6">
          <Link href="/" className="flex items-center space-x-2">
            <span className="font-semibold text-lg tracking-tight">PIRCOS</span>
            <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium tracking-wide text-muted-foreground uppercase">
              OS
            </span>
          </Link>

          {/* Navigation links */}
          <nav className="hidden md:flex items-center space-x-1">
            {navItems.map((item) => {
              const Icon = item.icon;
              const isActive = pathname === item.href;
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={cn(
                    "flex items-center space-x-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                    isActive
                      ? "bg-secondary text-foreground font-semibold"
                      : "text-muted-foreground hover:bg-muted/50 hover:text-foreground"
                  )}
                >
                  <Icon className="h-3.5 w-3.5" />
                  <span>{item.label}</span>
                </Link>
              );
            })}
          </nav>
        </div>

        {/* Controls: Dual View Toggle + Theme toggle */}
        <div className="flex items-center space-x-3">
          {/* Dual Valuation Toggle (Custo vs Mercado) */}
          {onValuationModeChange && (
            <div className="flex items-center rounded-lg border border-border bg-secondary/50 p-0.5 text-xs">
              <button
                type="button"
                onClick={() => onValuationModeChange("market")}
                className={cn(
                  "flex items-center space-x-1 rounded-md px-2.5 py-1 font-medium transition-all",
                  valuationMode === "market"
                    ? "bg-card text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                <TrendingUp className="h-3 w-3" />
                <span>Mercado</span>
              </button>
              <button
                type="button"
                onClick={() => onValuationModeChange("cost")}
                className={cn(
                  "flex items-center space-x-1 rounded-md px-2.5 py-1 font-medium transition-all",
                  valuationMode === "cost"
                    ? "bg-card text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                <DollarSign className="h-3 w-3" />
                <span>Custo (PM)</span>
              </button>
            </div>
          )}

          {onValuationModeChange && valuationMode === "market" && lastUpdated && (
            <span className="hidden sm:inline-flex items-center text-[10px] text-muted-foreground">
              <Clock className="h-3 w-3 mr-1" />
              {new Date(lastUpdated).toLocaleString("pt-BR", {
                day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit"
              })}
            </span>
          )}

          {/* Theme switch button */}
          {mounted && (
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 text-muted-foreground hover:text-foreground"
              onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
              aria-label="Toggle theme"
            >
              {theme === "dark" ? (
                <Sun className="h-4 w-4" />
              ) : (
                <Moon className="h-4 w-4" />
              )}
            </Button>
          )}
        </div>
      </div>
    </header>
  );
}
