# PIRCOS — Progresso de Tarefas

## Status Atual

- **Tarefa Ativa**: Todas as Fases (1 a 6) Concluídas com Sucesso! 🚀
- **Próximo Passo**: Sistema 100% operacional em Modo 1 (Docker Compose) e Modo 2 (Híbrido) pronto para homologação e uso.

---

## Fase 1: Setup e Infraestrutura Base ✅

- [x] **1.1** Criar estrutura de diretórios `api/` e `web/`
- [x] **1.2** Criar `docker-compose.yml`, `.env.example`, `api/Dockerfile` e `web/Dockerfile` conforme Seção 8
- [x] **1.3** Implementar migrations DDL no banco de dados
- [x] **1.4** Criar conexão de banco de dados em Go usando `pgx/v5` (+ config, domain structs, main.go básico)
- [x] **1.5** Validar `docker compose up --build` (Modo 1) e Modo 2 híbrido

## Fase 2: Serviços de Mercado e Dados Externos ✅

- [x] **2.1** Implementar cliente Go para Yahoo Finance Chart API (cotações com fallback)
- [x] **2.2** Implementar cliente Go para Banco Central (SGS — CDI, Selic, IPCA)
- [x] **2.3** Criar rotina de persistência e fallback local para dados de mercado offline

## Fase 3: Motor de Cálculo Contábil & IR ✅

- [x] **3.1** Implementar serviço de Preço Médio Ponderado
- [x] **3.2** Implementar agrupador e identificador de Swing Trade vs Day Trade
- [x] **3.3** Implementar regras de baldes de IR (Ações, FIIs, isenção 20k, compensação de prejuízo)
- [x] **3.4** Implementar motor de recálculo em cascata com snapshots mensais

## Fase 4: Parser Estático de Notas & Storage ✅

- [x] **4.1** Implementar handler de upload e storage local de PDFs
- [x] **4.2** Implementar extração de texto e regex para o padrão Sinacor B3
- [x] **4.3** Criar endpoints de conferência e confirmação de nota

## Fase 5: API REST Completa ✅

- [x] **5.1** Expor endpoints de CRUD de transações, proventos e ativos
- [x] **5.2** Expor endpoints de relatórios de carteira e drill-down de IR

## Fase 6: Frontend (Next.js & shadcn/ui) ✅

- [x] **6.1** Setup do Next.js, Tailwind, dark mode e componentes shadcn/ui
- [x] **6.2** Construir Dashboard com patrimônio dual (custo vs mercado) e gráficos de benchmark
- [x] **6.3** Construir tela de Ingestão de Notas (split screen PDF + form)
- [x] **6.4** Construir tela de Prévia do IR com cards mensais e modal de drill-down

---

## Observações Técnicas

### Fase 1
- **Go**: v1.26.8, **Node**: v24.11.1, **Docker**: 28.4.0, **Compose**: v2.39.2
- **Dockerfiles**: Usam `golang:1.26-alpine` e `node:24-alpine` compatíveis com versões locais.
- **Dependências Go**: `pgx/v5 v5.11.0`, `shopspring/decimal v1.4.0`
- **Migração**: Runner próprio (sem frameworks), idempotente com `IF NOT EXISTS` e tabela `schema_migrations`.
- **Modo 2 validado**: `docker compose up -d postgres` + `go run ./cmd/server/main.go` — conexão, migrations e health endpoint OK.
- **Modo 1**: Validação completa do `docker compose up --build` adiada para após Fase 6 (Next.js precisa de projeto inicializado). API container funcional.
- **Endpoints ativos**: `GET /health` (retorna status DB), `GET /api/v1/` (info do serviço).
- **Compilação**: `go build ./...` e `go vet ./...` passam sem erros.

### Fase 2
- **Yahoo Finance**: `service/quotes.go` — client HTTP com conversão automática de tickers BR (.SA suffix), timeout 10s.
- **Binance**: Integrado no mesmo `quotes.go` para cotações crypto (par USDT automático).
- **BCB SGS**: `service/benchmarks.go` — séries 11/12/433, datas DD/MM/YYYY, fetch concorrente das 3 séries.
- **Persistência**: `repository/market_quotes.go` e `repository/macro_benchmarks.go` com upsert via ON CONFLICT.
- **Fallback**: `service/market_data.go` orquestra: tenta API → cacheia → se falhar, usa cache local.
- **Sync incremental**: `SyncBenchmarks()` busca apenas a partir da última data conhecida.
- **Testes**: 21 testes passando (ticker conversion, BCB date parsing, series mapping).

### Fase 3
- **Preço Médio Ponderado**: `service/average_price.go` — cálculo determinístico com `shopspring/decimal`. Suporta BUY, SELL, SPLIT, GROUPING, BONUS e amortização de FII.
- **Classificador de Trades**: `service/trade_classifier.go` — pareamento FIFO de Day Trade por data fiscal (`DATE(operation_date)`) e ativo. Separação precisa de frações remanescentes para Swing Trade.
- **Regras de IR**: `service/tax_engine.go` — baldes isolados (`STOCKS_SWING`, `STOCKS_DAYTRADE`, `FII`, `ETF`, etc.), alíquotas de 15% e 20%, isenção de R$ 20.000,00 para swing em ações, carregamento perpétuo de prejuízos e abatimento de IRRF.
- **Recálculo em Cascata**: `service/cascade_engine.go` — motor cronológico que processa proventos, apura resultados e gera snapshots mensais persistidos via `repository/monthly_tax_balance.go`.
- **Testes**: 25 testes unitários cobrindo todos os cenários fiscais e transacionais com 100% de sucesso.

### Fase 4
- **Storage Local**: `internal/storage/storage.go` — interface `FileStorage` com implementação `LocalStorage` salvando com prefixo único e limpeza automática em falhas.
- **Parser Sinacor**: `service/parser_sinacor.go` — extração de dados com biblioteca Go `dslipak/pdf`. Regexes robustas para datas, número de nota, itens de negociação (C/V, ativo, quantidade, preço, total) e despesas de corretagem/emolumentos com rateio proporcional.
- **Endpoints de Nota**: `internal/handler/documents.go` — `POST /api/v1/documents/parse` para conferência prévia sem persistência, e `POST /api/v1/documents/confirm` para arquivamento definitivo, gravação de ativos/transações e disparo automático do recálculo de IR em cascata.
- **Testes**: Testes de storage e handlers cobrindo parsing e validações.

### Fase 5
- **CRUD Completo**: `internal/handler/transactions.go`, `internal/handler/assets.go` e `internal/handler/earnings.go` expondo listagem, criação manual e filtros temporais/por ativo.
- **Validação de Portfólio**: `service/portfolio.go` e `internal/handler/portfolio.go` calculando posições ativas, custo total vs valor de mercado, alocação percentual por classe de ativos e timeline histórico de patrimônio.
- **Relatórios de IR & Drilldown**: `internal/handler/tax.go` fornecendo visão anual dos 12 meses com DARFs e status fiscais (`DUE`, `EXEMPT`, `NO_ACTIVITY`), e endpoint de drill-down com raio-X detalhado de todas as operações e custos do mês.
- **Benchmarks**: `internal/handler/benchmarks.go` expondo séries de mercado com filtros de data.
- **Testes**: 30 testes unitários passando em 100% de sucesso; compilação e `go vet` perfeitamente limpos.

### Fase 6
- **Next.js & Tailwind**: App Router com Next.js 15+, TypeScript, Tailwind CSS, Lucide Icons e `next-themes` (Dark Mode nativo).
- **Design System**: Estética Apple Minimalist (paleta monocromática com cinzas neutros, bordas sutis, tipografia limpa, sem cores saturadas). Componentes shadcn/ui customizados (`Button`, `Card`, `Badge`, `Input`).
- **Dashboard (`/`)**: 4 KPI cards (Patrimônio com switch Custo vs Mercado, Lucro/Prejuízo, Ativos sob Custódia, Alocação), gráfico de evolução histórica com Recharts (`PerformanceChart`) e tabela consolidada de custódia com percentuais de ganho/perda.
- **Ingestão de Notas (`/documents`)**: Layout Split Screen (visualizador nativo de PDF com drag-and-drop à esquerda, formulário de conferência detalhado da nota Sinacor à direita com recálculo automático de custos e persistência com 1 clique).
- **Prévia do IR (`/tax`)**: Grid anual com 12 cards mensais exibindo total de vendas, base tributável, valor de DARF e badges de status (`A PAGAR`, `ISENTO`, `SEM MOVIMENTO`), além de Modal de Drill-down para auditoria detalhada de cada balde e compensação de prejuízo.
- **Histórico de Transações (`/transactions`)**: Visualização paginada e filtrável por ativo/tipo com registro temporal de todas as ordens.
- **Docker Compose (Modo 1)**: Build e deploy com sucesso de todos os serviços (`pircos-postgres`, `pircos-api` e `pircos-web`) com rede isolada, healthchecks e comunicação ponta a ponta validada.
