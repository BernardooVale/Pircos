# PIRCOS — Patrimônio, IR e OS

> Sistema autônomo e local de gestão de investimentos, consolidação patrimonial, custódia e conferência de notas de corretagem (PDF Sinacor) e apuração contábil determinística de Imposto de Renda sobre renda variável e multimercado.

---

## 📌 Sumário

- [Visão Geral](#-visão-geral)
- [Tech Stack & Arquitetura](#-tech-stack--arquitetura)
- [Decisões e Escolhas de Implementação](#-decisões-e-escolhas-de-implementação)
- [Estrutura do Repositório](#-estrutura-do-repositório)
- [Funcionalidades Implementadas](#-funcionalidades-implementadas)
- [Como Rodar o Projeto](#-como-rodar-o-projeto)
  - [Modo 1: Execução Total em Contêineres (Docker Compose)](#modo-1-execução-total-em-contêineres-docker-compose)
  - [Modo 2: Desenvolvimento Híbrido (Host + Docker DB)](#modo-2-desenvolvimento-híbrido-host--docker-db)
- [Endpoints da API REST](#-endpoints-da-api-rest)
- [Testes e Validação](#-testes-e-validação)

---

## 🔭 Visão Geral

O **Pircos** foi projetado para resolver a complexidade de apuração fiscal e acompanhamento de carteira no mercado de capitais brasileiro (B3) e internacional, mantendo privacidade absoluta dos dados, controle local em banco relacional, zero custo de APIs pagas e consistência contábil rigorosa.

O sistema elimina o risco de imprecisão de cálculos fiscais através de aritmética de ponto fixo arbitrário e fornece uma experiência refinada baseada na filosofia de design **Apple Minimalist**.

---

## 🛠 Tech Stack & Arquitetura

### Backend
- **Go 1.26+**: Alta performance, concorrência nativa, tipagem estática e binário único compilado.
- **Roteamento HTTP**: `net/http` padrão com pattern matching introduzido no Go 1.22+ (`GET /path`, `POST /path`) e middleware nativo de CORS, sem dependências infladas de frameworks pesados.
- **Driver PostgreSQL**: `jackc/pgx/v5` utilizando pool de conexões de alto desempenho (`pgxpool.Pool`) com queries SQL puras (sem ORM para garantir previsibilidade e indexação ideal).
- **Aritmética Determinística**: `shopspring/decimal` em 100% dos cálculos fiscais e patrimoniais (**zero** uso de `float64` em operações contábeis para prevenir erros de precisão IEEE 754).
- **Parser de Documentos**: `github.com/dslipak/pdf` (fork moderno e estável de `rsc/pdf`) para extração estruturada de texto de arquivos PDF.

### Frontend
- **Next.js 15+ (App Router)**: Renderização híbrida com suporte a Server/Client Components e output estático otimizado (`standalone`).
- **React 19 & TypeScript 5+**: Tipagem ponta a ponta e controle estrito de contratos de API.
- **Tailwind CSS & Design System**: Estética Apple Minimalist, paleta neutra monocromática (cinzas neutros, zinc/neutral, bordas sutis) e tipografia fluida baseada em SF Pro / Inter.
- **Dark Mode Nativo**: Suporte imediato a alternância claro/escuro via `next-themes`.
- **Visualização de Dados**: `recharts` para curvas de evolução patrimonial e alocação por classes de ativos (Donut Chart).
- **Ícones**: `lucide-react`.

### Armazenamento & Infraestrutura
- **Banco de Dados**: PostgreSQL 16 Alpine com extensão `pgcrypto` para geração nativa de UUIDs v4 e tipos de precisão monetária `NUMERIC(18, 4)`, `NUMERIC(24, 8)` e `NUMERIC(12, 6)`.
- **Storage Local de Arquivos**: Interface abstrata `FileStorage` com implementação `LocalStorage` salvando notas com hash e UUID anti-colisão em disco local (`storage/documents/`).
- **Containerização**: Multi-stage Dockerfiles com imagens mínimas (`alpine:3.20` para a API Go e `node:24-alpine` para o Next.js) orquestradas via `docker-compose.yml`.

### Fontes de Dados Externas (Zero Chaves / Sem Tokens Pagos)
- **Cotações de Ações/FIIs/ETFs**: Yahoo Finance Chart API (`query1.finance.yahoo.com/v8/finance/chart/{TICKER}`) com conversão automática de tickers brasileiros para o sufixo `.SA` e cache local em banco (`market_quotes`) com estratégia offline fallback.
- **Criptoativos**: Binance Public API (`api.binance.com/api/v3/ticker/price`) com conversão para par USDT.
- **Séries Macroeconômicas (Banco Central do Brasil)**: API pública BCB SGS para séries 11 (Selic diária), 12 (CDI diário) e 433 (IPCA mensal), sincronizadas concorrentemente e armazenadas em `macro_benchmarks`.

---

## 🧠 Decisões e Escolhas de Implementação

### 1. Ausência de Ponto Flutuante (`float64`)
Cálculos de Imposto de Renda sobre ações e fundos imobiliários exigem precisão exata de centavos para geração de DARFs e apuração de prejuízos acumulados. Tipos `float64` sofrem com resíduos de precisão binária (ex: `0.1 + 0.2 != 0.3`). Toda a árvore contábil do Pircos opera via structs `decimal.Decimal` com arredondamento bancário (`RoundBank`) apenas na consolidação final.

### 2. Preço Médio Ponderado (PM) e Eventos Corporativos
Implementado em `api/internal/service/average_price.go`:
- **Compra (`BUY`)**:
  $$\text{Novo PM} = \frac{(\text{Qtd Atual} \times \text{PM Atual}) + (\text{Qtd Comprada} \times \text{Preço Unit}) + \text{Custos}}{\text{Qtd Atual} + \text{Qtd Comprada}}$$
- **Venda (`SELL`)**: O Preço Médio não é alterado na venda; apenas deduz-se a quantidade em custódia. O resultado financeiro apurado é:
  $$\text{Lucro/Prejuízo} = (\text{Valor Bruto Venda} - \text{Custos}) - (\text{Qtd Vendida} \times \text{PM})$$
- **Amortização de Cotas de FII**: O valor amortizado reduz diretamente o custo de aquisição (PM por cota), sem tributação imediata de ganho de capital até o esgotamento do PM.
- **Desdobramentos (`SPLIT`), Agrupamentos (`GROUPING`) e Bonificações (`BONUS`)**: Ajuste proporcional imediato sem impactar o capital total investido.

### 3. Classificação FIFO de Day Trade vs Swing Trade
Conforme as normas da Receita Federal do Brasil, operações de compra e venda do mesmo ativo no mesmo dia fiscal (`DATE(operation_date)`) configuram **Day Trade** e possuem regras tributárias distintas (alíquota de 20%, sem isenção de R$ 20.000). O serviço `trade_classifier.go`:
- Pareia compras e vendas do mesmo dia através de fila FIFO intradiária.
- Isola o resultado do Day Trade e encaminha eventuais sobras de quantidade para a esteira contábil de Swing Trade.

### 4. Baldes Tributários Isolados e Compensação Cruzada
Implementado em `api/internal/service/tax_engine.go` e `cascade_engine.go`:
- **FIIs**: Balde 100% segregado, alíquota de 20%, sem isenção de volume. Prejuízo de FII **só** compensa ganho de FII.
- **Ações Swing Trade**: Alíquota de 15%. Isenção de imposto sobre ganho de capital caso o somatório das vendas de ações no mês seja $\le \text{R\$} 20.000,00$. Prejuízos gerados em meses isentos continuam acumulando para compensação em meses tributáveis futuros.
- **ETFs / Ações Swing**: Compensação cruzada permitida pela legislação entre Swing Trade de ações e ETFs (alíquota de 15%).
- **Day Trade**: Alíquota de 20%, compensável com prejuízos de Day Trade.
- **Carregamento Perpétuo de Prejuízos**: Saldos negativos são carregados mês a mês indefinidamente até total absorção.
- **Abatimento de IRRF**: Dedução automática de Imposto de Renda Retido na Fonte (dedo-duro) sobre o DARF final calculado.

### 5. Motor de Recálculo em Cascata
Ao inserir, retificar ou deletar uma transação com data no passado, ou confirmar uma nova nota antiga, o `CascadeEngine` recalcula cronologicamente todos os meses subsequentes, propagando novos preços médios, resultados de vendas, compensações de prejuízo e regerando os snapshots na tabela `monthly_tax_balance`.

### 6. Fluxo de Ingestão de Notas Sinacor em Duas Etapas
1. **Prévia (`POST /api/v1/documents/parse`)**: O backend faz o parsing em memória via regex do padrão Sinacor B3 (detecta cabeçalho, número de nota, data de pregão, corretora, itens de compra/venda e rateia taxas/emolumentos proporcionalmente ao volume de cada ordem). Devolve JSON para a tela.
2. **Revisão e Confirmação (`POST /api/v1/documents/confirm`)**: A tela exibe o PDF lado a lado com um formulário editável. Após a validação do usuário, o arquivo é arquivado no storage local, os ativos e transações são persistidos em lote em transação ACID e o recálculo em cascata é acionado.

---

## 📁 Estrutura do Repositório

```text
Pircos/
├── api/                             # Backend em Go
│   ├── cmd/
│   │   └── server/
│   │       └── main.go              # Ponto de entrada, injeção de dependências e graceful shutdown
│   ├── internal/
│   │   ├── config/                  # Carregamento de variáveis de ambiente
│   │   ├── domain/                  # Entidades de domínio (Transaction, Asset, MonthlyTaxBalance, etc.)
│   │   ├── handler/                 # Handlers REST (Documents, Transactions, Tax, Portfolio, Benchmarks)
│   │   ├── repository/              # Acesso a dados via pgx/v5 (Assets, Documents, Transactions, Tax, Quotes)
│   │   ├── service/                 # Regras de negócio (AveragePrice, TradeClassifier, TaxEngine, Cascade, Parser)
│   │   └── storage/                 # Implementação de storage local para guarda de PDFs
│   ├── migrations/
│   │   └── 001_initial_schema.sql   # DDL relacional idempotente com 8 tabelas e índices
│   ├── Dockerfile                   # Multi-stage build Go (golang:1.26-alpine -> alpine:3.20)
│   ├── go.mod
│   └── go.sum
├── web/                             # Frontend Next.js 15+ (App Router)
│   ├── src/
│   │   ├── app/
│   │   │   ├── page.tsx             # Dashboard principal (KPIs, Dual Valuation, Gráficos, Custódia)
│   │   │   ├── documents/page.tsx   # Split Screen: Ingestor de PDF + Formulário Sinacor
│   │   │   ├── tax/page.tsx         # Painel de IR: 12 cards anuais de DARF + Modal Drilldown
│   │   │   ├── transactions/page.tsx# Livro-razão e histórico de transações
│   │   │   ├── layout.tsx           # Layout raiz com ThemeProvider
│   │   │   └── globals.css          # Design tokens Apple Minimalist e variáveis CSS
│   │   ├── components/
│   │   │   ├── header.tsx           # Navegação principal + Switch Custo vs Mercado + Theme Toggle
│   │   │   ├── dashboard/
│   │   │   │   └── performance-chart.tsx # Gráfico Recharts com linhas de Benchmark (CDI/Selic/IPCA)
│   │   │   └── ui/                  # Componentes base (Button, Card, Badge, Input)
│   │   └── lib/
│   │       ├── api.ts               # Cliente HTTP tipado para todos os endpoints da API
│   │       └── utils.ts             # Formatadores de moeda (BRL), percentual, data e classes CSS
│   ├── Dockerfile                   # Multi-stage build Next.js (deps -> builder -> runner standalone)
│   ├── next.config.mjs              # Configuração com output: 'standalone'
│   ├── package.json
│   ├── postcss.config.mjs
│   ├── tailwind.config.ts           # Configuração de temas, fontes do sistema e paletas
│   └── tsconfig.json
├── docker-compose.yml               # Orquestração do PostgreSQL, API Go e Next.js
├── .env.example                     # Modelo de variáveis de ambiente
├── AGENT_SPEC.md                    # Especificação técnica original do projeto
├── TASK_PROGRESS.md                 # Registro de execução das 6 fases
└── README.md                        # Documentação completa do projeto
```

---

## ⚡ Funcionalidades Implementadas

### 1. Dashboard com Validação Dual (`/`)
- **Toggle Custo vs Mercado**: Alternância imediata no cabeçalho entre o valor de aquisição histórico (PM acumulado) e o valor a mercado com badge de última atualização de cotações.
- **4 Cards de KPIs**: Patrimônio total, resultado não realizado (lucro/prejuízo monetário e percentual), total de ativos sob custódia e principal alocação percentual.
- **Gráfico de Evolução Patrimonial**: Plotagem temporal do patrimônio versus custo total e curvas comparativas de benchmarks macroeconômicos (CDI, Selic, IPCA).
- **Alocação por Classes de Ativos**: Donut Chart interativo em tons neutros (Ações, FIIs, FIAGRO, ETFs, Renda Fixa, Cripto, Internacional) com pesos percentuais.
- **Tabela Consolidada de Custódia**: Detalhamento por ticker com quantidade, preço médio, cotação atual, custo total, valor a mercado e resultado individual.

### 2. Ingestão Split-Screen de Notas de Corretagem (`/documents`)
- **Visualizador Integrado de PDF**: Renderização nativa do PDF à esquerda com drag-and-drop.
- **Formulário Dinâmico de Conferência à Direita**: Edição em tempo real das ordens extraídas (C/V, ticker, tipo, quantidade, preço unitário, total e flag de Day Trade) e de custos totais de liquidação/emolumentos/corretagem.
- **Conferência em 1 Clique**: Validação cruzada dos totais da nota e gravação definitiva com recálculo automático da carteira.

### 3. Painel de Imposto de Renda & Drilldown (`/tax`)
- **Seletor de Ano Fiscal**: Navegação anual com totais acumulados de imposto devido e DARFs gerados.
- **Grid de 12 Meses**: Cards individuais para cada mês indicando:
  - Volume total de vendas;
  - Base tributável consolidada;
  - Imposto devido apurado;
  - Valor final do DARF a recolher;
  - Badges de status: `A PAGAR` (DARF > R$ 10,00), `ISENTO` (vendas de ações $\le$ R$ 20k e sem DARF) ou `SEM MOVIMENTO`.
- **Modal de Auditoria (Drilldown Fiscal)**: Raio-X completo com listagem itemizada de todas as operações do mês (incluindo Day Trades), custos rateados, lucros/prejuízos por ordem e histórico de perdas anteriores compensadas.

### 4. Histórico de Transações (`/transactions`)
- Livro-razão completo de movimentações com filtros por ativo e tipo de operação (`BUY`, `SELL`, `SPLIT`, `GROUPING`, `BONUS`).

---

## 🚀 Como Rodar o Projeto

### Pré-requisitos
- **Git**
- **Docker** e **Docker Compose** (para o Modo 1)
- **Go 1.26+** e **Node.js 24+** (caso utilize o Modo 2 para desenvolvimento local)

---

### Modo 1: Execução Total em Contêineres (Docker Compose)

Ideal para usuários finais ou deploy em servidores locais/VPS com zero dependências instaladas no sistema hospedeiro.

1. **Clone o repositório:**
   ```bash
   git clone https://github.com/BernardooVale/Pircos.git
   cd Pircos
   ```

2. **Configure as variáveis de ambiente:**
   ```bash
   cp .env.example .env
   ```

3. **Suba todo o ecossistema:**
   ```bash
   docker compose up --build -d
   ```

4. **Acesse as aplicações:**
   - **Frontend (Next.js)**: [http://localhost:3000](http://localhost:3000)
   - **API Backend (Go)**: [http://localhost:8080](http://localhost:8080)
   - **Healthcheck**: [http://localhost:8080/health](http://localhost:8080/health)

---

### Modo 2: Desenvolvimento Híbrido (Host + Docker DB)

Ideal para desenvolvimento ativo com hot-reload e compilação instantânea.

1. **Suba apenas o banco PostgreSQL via Docker Compose:**
   ```bash
   docker compose up -d postgres
   ```

2. **Inicie o Backend em Go (Terminal 1):**
   ```bash
   cd api
   # As migrações do schema são executadas automaticamente na inicialização
   go run ./cmd/server/main.go
   ```
   *O backend inicializará na porta `:8080` conectado ao PostgreSQL em `localhost:5432`.*

3. **Inicie o Frontend Next.js (Terminal 2):**
   ```bash
   cd web
   npm install
   npm run dev
   ```
   *O frontend inicializará em [http://localhost:3000](http://localhost:3000) comunicando-se com a API local.*

---

## 📡 Endpoints da API REST

A API do Pircos está versionada sob o prefixo `/api/v1`:

| Método | Endpoint | Descrição |
|---|---|---|
| `GET` | `/health` | Healthcheck do serviço e conexão com o banco de dados |
| `POST` | `/api/v1/documents/parse` | Recebe PDF multipart e devolve JSON com as ordens para conferência prévia |
| `POST` | `/api/v1/documents/confirm` | Persiste PDF no storage, grava transações e aciona recálculo em cascata |
| `GET` | `/api/v1/transactions` | Lista transações com filtros por `asset_id`, `operation_type`, datas |
| `POST` | `/api/v1/transactions` | Cadastro manual de transação avulsa |
| `GET` | `/api/v1/portfolio/summary` | Sumário de carteira (custo total, valor a mercado, P/L e alocação por classe) |
| `GET` | `/api/v1/portfolio/history` | Série histórica do patrimônio enriquecida com taxas de benchmarks |
| `GET` | `/api/v1/tax/monthly` | Painel anual de 12 meses com cálculo de IR e DARF (`?year=YYYY`) |
| `GET` | `/api/v1/tax/drilldown` | Raio-X fiscal do mês por balde (`?year_month=YYYY-MM&bucket=BUCKET`) |
| `POST` | `/api/v1/earnings` | Cadastro de proventos (`DIVIDEND`, `JCP`, `AMORTIZATION`, `INCOME`) |
| `GET` | `/api/v1/benchmarks` | Histórico de CDI, Selic e IPCA (suporta `?normalized=true` para base 100) |
| `GET` | `/api/v1/assets` | Lista ativos cadastrados no sistema |
| `POST` | `/api/v1/assets` | Cadastro avulso de ativo |

---

## 🧪 Testes e Validação

O projeto conta com ampla cobertura de testes unitários na esteira contábil e de parsing:

### Executando os testes do Backend
```bash
cd api
go test -v ./...
```
*Cobertura inclui:*
- `average_price_test.go`: Validação de compras múltiplas, vendas parciais/totais, desdobramentos, amortizações de FII e diluição por bonificação.
- `trade_classifier_test.go`: Pareamento FIFO de Day Trades intradiários e segregação de frações remanescentes para Swing Trade.
- `tax_engine_test.go`: Alíquotas de 15% e 20%, limite de isenção de R$ 20k, isolamento de baldes e compensação de prejuízos.
- `cascade_engine_test.go`: Recálculo cronológico em cascata e compensação cruzada entre Ações Swing e ETFs.
- `parser_sinacor_test.go`: Validação de regexes de notas e rateio proporcional de custos.
- `documents_test.go`, `crud_test.go`, `reports_test.go`: Handlers HTTP e persistência.

### Validando a compilação do Frontend
```bash
cd web
npm run build
```
*Executa validação estrita de tipos TypeScript, linting e geração de rotas estáticas otimizadas.*

---

## 📄 Licença

Distribuído sob a licença MIT. Consulte `LICENSE` para mais informações.
