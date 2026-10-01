# PIRCOS — Especificação Técnica e Guia de Execução Autônoma

## 1. Visão Geral

* **Nome**: Pircos (Patrimônio, IR e OS).
* **Propósito**: Sistema autônomo e local de gestão de investimentos, evolução patrimonial, custódia de PDFs de corretagem e cálculo determinístico de Imposto de Renda.
* **Usuário**: Monousuário inicial, modelado com chave de usuário (`user_id`) para futura expansão SaaS/mobile.
* **Stack**:
  * **Backend**: Go 1.26+ (`net/http` ou `go-chi/chi/v5`, `jackc/pgx/v5`, `shopspring/decimal`).
  * **Frontend**: Next.js 15+ (App Router, TypeScript, Tailwind CSS, shadcn/ui, Recharts).
  * **Runtime Frontend**: Node.js 24 (LTS) e npm 11+.
  * **Banco**: PostgreSQL 16+.
  * **Armazenamento de Arquivos**: Sistema de arquivos local (`./storage/documents/`) com abstração de interface.
  * **Integrações Externas (Sem tokens/chaves)**:
    * Cotações: Yahoo Finance Chart API (`query1.finance.yahoo.com/v8/finance/chart/{TICKER}`).
    * Cripto: Binance Public API (`api.binance.com/api/v3/ticker/price`).
    * Benchmarks Macroeconômicos: Banco Central do Brasil (SGS - Séries 11 Selic, 12 CDI, 433 IPCA).

---

## 2. Protocolo de Execução do Agente (Controle de Estado)

Devido à extensão do projeto e limite de contexto/tokens por sessão, o agente executor DEVE seguir rigorosamente este ciclo operacional:

1. **Arquivo de Estado**: Manter e atualizar o arquivo `TASK_PROGRESS.md` na raiz do projeto.
2. **Formato do `TASK_PROGRESS.md`**:
   * Cada tarefa deve ter: `[ ]` (Pendente), `[-]` (Em Andamento), `[x]` (Concluída).
   * Seção "Contexto Atual": Último arquivo modificado, decisões tomadas e próximo passo exato.
3. **Ciclo por Interação**:
   1. Ler `TASK_PROGRESS.md`.
   2. Identificar a próxima tarefa pendente.
   3. Marcar como `[-]`.
   4. Implementar código e testes daquela tarefa específica.
   5. Validar compilação/execução.
   6. Marcar como `[x]` e registrar observações técnicas no arquivo.
   7. Nunca tentar implementar múltiplos módulos complexos num único turno.
4. **Sub-Tarefas**: O Agente pode criar sub-tarefas para facilitar o desenvolvimento, mas fica totalmente sob escolha do mesmo.

---

## 3. Modelo de Dados (PostgreSQL)

Usar estritamente precisão decimal (`NUMERIC`), nunca tipos de ponto flutuante (`FLOAT`/`REAL`).

### 3.1 DDL Essencial

```sql
-- Extensão para UUIDs
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Usuários (preparo multi-tenant)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Ativos
-- Classes: 'STOCKS', 'FII', 'FIAGRO', 'ETF', 'FIXED_INCOME', 'CRYPTO', 'INTERNATIONAL'
CREATE TABLE assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticker VARCHAR(30) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    asset_class VARCHAR(30) NOT NULL,
    currency VARCHAR(10) DEFAULT 'BRL',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Documentos / PDFs anexos
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    original_filename VARCHAR(255) NOT NULL,
    storage_path VARCHAR(500) NOT NULL,
    file_size_bytes BIGINT NOT NULL,
    mime_type VARCHAR(100) DEFAULT 'application/pdf',
    uploaded_at TIMESTAMPTZ DEFAULT NOW()
);

-- Transações
-- Tipos: 'BUY', 'SELL', 'SPLIT', 'GROUPING', 'BONUS'
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    asset_id UUID NOT NULL REFERENCES assets(id),
    document_id UUID REFERENCES documents(id),
    operation_type VARCHAR(20) NOT NULL,
    quantity NUMERIC(24, 8) NOT NULL,
    unit_price NUMERIC(18, 4) NOT NULL,
    costs NUMERIC(18, 4) DEFAULT 0,
    total_amount NUMERIC(18, 4) NOT NULL,
    operation_date TIMESTAMPTZ NOT NULL,
    settlement_date DATE,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_transactions_user_asset_date
ON transactions(user_id, asset_id, operation_date ASC);

-- Proventos
-- Tipos: 'DIVIDEND', 'JCP', 'AMORTIZATION', 'INCOME'
CREATE TABLE earnings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    asset_id UUID NOT NULL REFERENCES assets(id),
    earning_type VARCHAR(30) NOT NULL,
    com_date DATE NOT NULL,
    payment_date DATE NOT NULL,
    gross_amount NUMERIC(18, 4) NOT NULL,
    tax_withheld NUMERIC(18, 4) DEFAULT 0,
    net_amount NUMERIC(18, 4) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Snapshots Mensais de IR e Fechamento
-- asset_bucket: 'STOCKS_SWING', 'STOCKS_DAYTRADE', 'FII', 'ETF', 'CRYPTO', 'INTERNATIONAL'
CREATE TABLE monthly_tax_balance (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    year_month CHAR(7) NOT NULL, -- 'YYYY-MM'
    asset_bucket VARCHAR(30) NOT NULL,
    total_sales NUMERIC(18, 4) DEFAULT 0,
    gross_profit NUMERIC(18, 4) DEFAULT 0,
    losses_deducted NUMERIC(18, 4) DEFAULT 0,
    accumulated_loss_carried NUMERIC(18, 4) DEFAULT 0,
    taxable_base NUMERIC(18, 4) DEFAULT 0,
    tax_rate NUMERIC(5, 4) NOT NULL,
    tax_due NUMERIC(18, 4) DEFAULT 0,
    irrf_withheld NUMERIC(18, 4) DEFAULT 0,
    final_darf NUMERIC(18, 4) DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uk_user_month_bucket UNIQUE(user_id, year_month, asset_bucket)
);

-- Cache de Cotações Locais (Fallback Offline)
CREATE TABLE market_quotes (
    ticker VARCHAR(30) PRIMARY KEY,
    price NUMERIC(18, 4) NOT NULL,
    currency VARCHAR(10) DEFAULT 'BRL',
    updated_at TIMESTAMPTZ NOT NULL
);

-- Benchmarks Macroeconômicos (Histórico)
CREATE TABLE macro_benchmarks (
    series_name VARCHAR(20) NOT NULL, -- 'CDI', 'SELIC', 'IPCA'
    reference_date DATE NOT NULL,
    rate_value NUMERIC(12, 6) NOT NULL,
    PRIMARY KEY(series_name, reference_date)
);
```

---

## 4. Regras de Negócio e Cálculos

### 4.1 Preço Médio Ponderado (PM)

* **Compra**:

  $$\text{Novo PM} = \frac{(\text{Qtd Atual} \times \text{PM Atual}) + (\text{Qtd Comprada} \times \text{Preço Unit}) + \text{Custos}}{\text{Qtd Atual} + \text{Qtd Comprada}}$$

* **Venda**:
  * PM não sofre alteração.
  * Resultado: $\text{Lucro/Prejuízo} = (\text{Valor Bruto Venda} - \text{Custos}) - (\text{Qtd Vendida} \times \text{PM})$.
  * Redução apenas da quantidade de cotas/ações em custódia.
* **Amortização de FII**:
  * Valor amortizado por cota abate diretamente o Preço Médio.

### 4.2 Apuração de Imposto de Renda

1. **Identificação de Day Trade**:
   * Ocorre quando há operação de compra e venda do mesmo ativo no mesmo dia fiscal (`DATE(operation_date)` idêntico).
   * Alíquota: 20%. Sem faixa de isenção.
2. **Baldes Tributários Isolados**:
   * Prejuízo de FII **só** compensa lucro de FII (alíquota 20%).
   * Prejuízo de Ações Swing Trade compensa lucro de Swing Trade de Ações/ETFs (alíquota 15%).
   * Prejuízos são carregados indefinidamente até serem abatidos integralmente.
3. **Isenção de R$ 20.000 em Ações**:
   * Somatório de vendas de Ações (Swing Trade) $\le$ R$ 20.000 no mês calendário torna o lucro **isento**.
   * Se ultrapassar R$ 20.000,01, o imposto incide sobre todo o lucro líquido do mês (descontando prejuízos acumulados anteriores).
   * Prejuízos gerados em mês com vendas abaixo de R$ 20.000 acumulam normalmente para compensação futura.
4. **Recálculo em Cascata**:
   * Inserção, edição ou exclusão de transação com data passada invalida e recalcula cronologicamente todos os meses subsequentes em `monthly_tax_balance`.

### 4.3 Ingestão Estática de PDF

* Biblioteca Go para extração de texto estruturado (`ledongthang/pdf` ou `pdfcpu`).
* **Parser Padrão Sinacor (B3)**:
  * Expressões regulares para capturar linhas de negociação: `C/V`, Ticker/Ativo, Quantidade, Preço, Valor Total.
  * Expressões regulares no rodapé: Total de custos (taxa de liquidação, emolumentos, corretagem).
  * Rateio de custos entre as operações da mesma nota proporcionalmente ao volume financeiro.
* **Fluxo de API**: O backend processa o arquivo temporário, extrai os campos e responde com JSON estruturado. A persistência só ocorre quando o frontend envia a confirmação após revisão do usuário.

---

## 5. Arquitetura do Backend em Go

### 5.1 Estrutura de Pastas

```text
api/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/             # Variáveis de ambiente (Porta, DB URL, Storage Path)
│   ├── domain/             # Structs de domínio e interfaces
│   ├── repository/         # Queries PostgreSQL usando pgx/v5 puro
│   ├── service/            # Lógica: tax_engine, parser_sinacor, quotes, benchmarks
│   ├── handler/            # Handlers HTTP REST e middlewares
│   └── storage/            # Gestão de arquivos em disco
├── migrations/             # Migrações SQL numeradas
├── go.mod
└── go.sum
```

### 5.2 Contrato de Endpoints Principais

* `POST /api/v1/documents/parse` -> Recebe PDF multipart, devolve transações detectadas para conferência.
* `POST /api/v1/documents/confirm` -> Salva documento no storage, persiste transações no banco e dispara recálculo.
* `GET /api/v1/transactions` -> Filtros por ativo, período e tipo de operação.
* `POST /api/v1/transactions` -> Cadastro manual de transação avulsa.
* `GET /api/v1/portfolio/summary` -> Custo total, valor atual a mercado, alocação por classe.
* `GET /api/v1/portfolio/history` -> Série temporal de patrimônio com suporte a comparação de benchmarks.
* `GET /api/v1/tax/monthly?year=YYYY` -> Prévia de IR, DARFs gerados e status de cada mês.
* `GET /api/v1/tax/drilldown?year_month=YYYY-MM&bucket=BUCKET` -> Raio-X detalhado com todas as vendas e custos do mês.
* `POST /api/v1/earnings` -> Cadastro manual de proventos.
* `GET /api/v1/benchmarks` -> Retorna séries normalizadas (CDI, Selic, IPCA).

---

## 6. Frontend Next.js & UI/UX

* **Estética**: Filosofia Apple Minimalist. Foco em tipografia refinada (SF Pro / Inter), paleta neutra (cinzas profundos no dark mode, brancos e grafites no light mode). Zero cores saturadas ou degradês neon.
* **Modo Noturno**: Suporte nativo e fluido via `next-themes` e Tailwind dark classes.
* **Visão Dual de Patrimônio**: Toggle simples na barra superior alternando entre:
  * Valor de Custo (PM acumulado).
  * Valor de Mercado (Cotações com badge de data/hora da última atualização).
* **Gráficos**:
  * Curva patrimonial normalizada comparando contra CDI, Selic, IPCA, Ibovespa e S&P 500.
  * Donut chart limpo com alocação por classes de ativos.
* **Módulo de Conferência de Notas**:
  * Divisão lado a lado: PDF renderizado à esquerda; formulário editável pré-preenchido pelo parser à direita.

---

## 7. Fases do Plano de Entrega (Roteiro para o Agente)

O agente deve transcrever estas fases para o arquivo `TASK_PROGRESS.md` antes de iniciar:

### Fase 1: Setup e Infraestrutura Base

* [ ] Criar estrutura de diretórios `api/` e `web/`.
* [ ] Criar `docker-compose.yml`, `.env.example`, `api/Dockerfile` e `web/Dockerfile` conforme Seção 8.
* [ ] Implementar migrations DDL no banco de dados.
* [ ] Criar conexão de banco de dados em Go usando `pgx/v5`.
* [ ] Validar `docker compose up --build` (Modo 1) e Modo 2 híbrido.

### Fase 2: Serviços de Mercado e Dados Externos

* [ ] Implementar cliente Go para Yahoo Finance Chart API (cotações com fallback).
* [ ] Implementar cliente Go para Banco Central (SGS - CDI, Selic, IPCA).
* [ ] Criar rotina de persistência e fallback local para dados de mercado offline.

### Fase 3: Motor de Cálculo Contábil & IR

* [ ] Implementar serviço de Preço Médio Ponderado.
* [ ] Implementar agrupador e identificador de Swing Trade vs Day Trade.
* [ ] Implementar regras de baldes de IR (Ações, FIIs, isenção 20k, compensação de prejuízo).
* [ ] Implementar motor de recálculo em cascata com snapshots mensais.

### Fase 4: Parser Estático de Notas & Storage

* [ ] Implementar handler de upload e storage local de PDFs.
* [ ] Implementar extração de texto e regex para o padrão Sinacor B3.
* [ ] Criar endpoints de conferência e confirmação de nota.

### Fase 5: API REST Completa

* [ ] Expor endpoints de CRUD de transações, proventos e ativos.
* [ ] Expor endpoints de relatórios de carteira e drill-down de IR.

### Fase 6: Frontend (Next.js & shadcn/ui)

* [ ] Setup do Next.js, Tailwind, dark mode e componentes shadcn/ui.
* [ ] Construir Dashboard com patrimônio dual (custo vs mercado) e gráficos de benchmark.
* [ ] Construir tela de Ingestão de Notas (split screen PDF + form).
* [ ] Construir tela de Prévia do IR com cards mensais e modal de drill-down.

---

## 8. Configuração de Contêineres e Ambiente

O Pircos usa Docker para execução "zero setup" em qualquer máquina com Docker instalado, e permite desenvolvimento local com hot-reload.

### 8.1 Estrutura de Arquivos de Infraestrutura

```text
pircos/
├── docker-compose.yml
├── .env.example
├── api/
│   └── Dockerfile
└── web/
    └── Dockerfile
```

### 8.2 `docker-compose.yml` (Raiz)

```yaml
services:
  postgres:
    image: postgres:16-alpine
    container_name: pircos-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: ${DB_USER:-pircos}
      POSTGRES_PASSWORD: ${DB_PASSWORD:-pircos_secret}
      POSTGRES_DB: ${DB_NAME:-pircos_db}
    ports:
      - "${DB_PORT:-5432}:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER:-pircos} -d ${DB_NAME:-pircos_db}"]
      interval: 5s
      timeout: 5s
      retries: 5

  api:
    build:
      context: ./api
      dockerfile: Dockerfile
    container_name: pircos-api
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      PORT: 8080
      DATABASE_URL: postgres://${DB_USER:-pircos}:${DB_PASSWORD:-pircos_secret}@postgres:5432/${DB_NAME:-pircos_db}?sslmode=disable
      STORAGE_DIR: /app/storage/documents
    ports:
      - "8080:8080"
    volumes:
      - ./api/storage/documents:/app/storage/documents

  web:
    build:
      context: ./web
      dockerfile: Dockerfile
    container_name: pircos-web
    restart: unless-stopped
    depends_on:
      - api
    environment:
      NEXT_PUBLIC_API_URL: http://localhost:8080
    ports:
      - "3000:3000"

volumes:
  postgres-data:
```

### 8.3 `api/Dockerfile` (Backend Go)

Multi-stage build para manter a imagem leve e segura:

```dockerfile
# Estágio de compilação
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server ./cmd/server/main.go

# Imagem final de execução
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/server /app/server
RUN mkdir -p /app/storage/documents

EXPOSE 8080

CMD ["/app/server"]
```

### 8.4 `web/Dockerfile` (Frontend Next.js)

```dockerfile
# Estágio de dependências
FROM node:24-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

# Estágio de build
FROM node:24-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

# Imagem final de execução
FROM node:24-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["node", "server.js"]
```

> **Nota para o Agente**: Ativar `output: "standalone"` no `next.config.js` / `next.config.mjs` para suporte ao build otimizado acima. Garantir que a pasta `web/public/` exista (mesmo vazia, com `.gitkeep`), senão o `COPY` do `public` falha.

### 8.5 `.env.example` (Raiz)

```env
# Banco de Dados
DB_USER=pircos
DB_PASSWORD=pircos_secret
DB_NAME=pircos_db
DB_PORT=5432

# URLs de Serviço
NEXT_PUBLIC_API_URL=http://localhost:8080
```

### 8.6 Instruções de Execução

#### Modo 1: Execução Total em Contêineres (Usuários / VPS)

Requer apenas Docker e Docker Compose:

```bash
# 1. Copiar variáveis de ambiente
cp .env.example .env

# 2. Subir todos os serviços (Banco, API Go e Next.js)
docker compose up --build
```

* Frontend: `http://localhost:3000`
* API Go: `http://localhost:8080`

#### Modo 2: Desenvolvimento Híbrido (Agente / Dev)

Mantém PostgreSQL no contêiner e executa código no host, para feedback e hot-reload rápidos:

```bash
# 1. Subir apenas o banco de dados
docker compose up -d postgres

# 2. Rodar a API Go localmente (terminal 1)
cd api
go run ./cmd/server/main.go

# 3. Rodar o Next.js localmente (terminal 2)
cd web
npm run dev
```