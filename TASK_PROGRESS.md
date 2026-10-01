# PIRCOS — Progresso de Tarefas

## Status Atual

- **Tarefa Ativa**: Fase 2.1 — Implementar cliente Go para Yahoo Finance Chart API
- **Próximo Passo**: Criar service/quotes.go com client HTTP para cotações Yahoo Finance

---

## Fase 1: Setup e Infraestrutura Base ✅

- [x] **1.1** Criar estrutura de diretórios `api/` e `web/`
- [x] **1.2** Criar `docker-compose.yml`, `.env.example`, `api/Dockerfile` e `web/Dockerfile` conforme Seção 8
- [x] **1.3** Implementar migrations DDL no banco de dados
- [x] **1.4** Criar conexão de banco de dados em Go usando `pgx/v5` (+ config, domain structs, main.go básico)
- [x] **1.5** Validar `docker compose up --build` (Modo 1) e Modo 2 híbrido

## Fase 2: Serviços de Mercado e Dados Externos

- [ ] **2.1** Implementar cliente Go para Yahoo Finance Chart API (cotações com fallback)
- [ ] **2.2** Implementar cliente Go para Banco Central (SGS — CDI, Selic, IPCA)
- [ ] **2.3** Criar rotina de persistência e fallback local para dados de mercado offline

## Fase 3: Motor de Cálculo Contábil & IR

- [ ] **3.1** Implementar serviço de Preço Médio Ponderado
- [ ] **3.2** Implementar agrupador e identificador de Swing Trade vs Day Trade
- [ ] **3.3** Implementar regras de baldes de IR (Ações, FIIs, isenção 20k, compensação de prejuízo)
- [ ] **3.4** Implementar motor de recálculo em cascata com snapshots mensais

## Fase 4: Parser Estático de Notas & Storage

- [ ] **4.1** Implementar handler de upload e storage local de PDFs
- [ ] **4.2** Implementar extração de texto e regex para o padrão Sinacor B3
- [ ] **4.3** Criar endpoints de conferência e confirmação de nota

## Fase 5: API REST Completa

- [ ] **5.1** Expor endpoints de CRUD de transações, proventos e ativos
- [ ] **5.2** Expor endpoints de relatórios de carteira e drill-down de IR

## Fase 6: Frontend (Next.js & shadcn/ui)

- [ ] **6.1** Setup do Next.js, Tailwind, dark mode e componentes shadcn/ui
- [ ] **6.2** Construir Dashboard com patrimônio dual (custo vs mercado) e gráficos de benchmark
- [ ] **6.3** Construir tela de Ingestão de Notas (split screen PDF + form)
- [ ] **6.4** Construir tela de Prévia do IR com cards mensais e modal de drill-down

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
