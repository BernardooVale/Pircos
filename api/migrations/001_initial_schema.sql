-- 001_initial_schema.sql
-- Pircos: Schema inicial com todas as tabelas do sistema
-- Executado automaticamente pelo servidor Go na inicialização

-- Extensão para UUIDs
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- Usuários (preparo multi-tenant)
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================================
-- Ativos
-- Classes: 'STOCKS', 'FII', 'FIAGRO', 'ETF', 'FIXED_INCOME', 'CRYPTO', 'INTERNATIONAL'
-- ============================================================
CREATE TABLE IF NOT EXISTS assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticker VARCHAR(30) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    asset_class VARCHAR(30) NOT NULL,
    currency VARCHAR(10) DEFAULT 'BRL',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================================
-- Documentos / PDFs anexos
-- ============================================================
CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    original_filename VARCHAR(255) NOT NULL,
    storage_path VARCHAR(500) NOT NULL,
    file_size_bytes BIGINT NOT NULL,
    mime_type VARCHAR(100) DEFAULT 'application/pdf',
    uploaded_at TIMESTAMPTZ DEFAULT NOW()
);

-- ============================================================
-- Transações
-- Tipos: 'BUY', 'SELL', 'SPLIT', 'GROUPING', 'BONUS'
-- ============================================================
CREATE TABLE IF NOT EXISTS transactions (
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

CREATE INDEX IF NOT EXISTS idx_transactions_user_asset_date
ON transactions(user_id, asset_id, operation_date ASC);

-- ============================================================
-- Proventos
-- Tipos: 'DIVIDEND', 'JCP', 'AMORTIZATION', 'INCOME'
-- ============================================================
CREATE TABLE IF NOT EXISTS earnings (
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

-- ============================================================
-- Snapshots Mensais de IR e Fechamento
-- asset_bucket: 'STOCKS_SWING', 'STOCKS_DAYTRADE', 'FII', 'ETF', 'CRYPTO', 'INTERNATIONAL'
-- ============================================================
CREATE TABLE IF NOT EXISTS monthly_tax_balance (
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

-- ============================================================
-- Cache de Cotações Locais (Fallback Offline)
-- ============================================================
CREATE TABLE IF NOT EXISTS market_quotes (
    ticker VARCHAR(30) PRIMARY KEY,
    price NUMERIC(18, 4) NOT NULL,
    currency VARCHAR(10) DEFAULT 'BRL',
    updated_at TIMESTAMPTZ NOT NULL
);

-- ============================================================
-- Benchmarks Macroeconômicos (Histórico)
-- ============================================================
CREATE TABLE IF NOT EXISTS macro_benchmarks (
    series_name VARCHAR(20) NOT NULL, -- 'CDI', 'SELIC', 'IPCA'
    reference_date DATE NOT NULL,
    rate_value NUMERIC(12, 6) NOT NULL,
    PRIMARY KEY(series_name, reference_date)
);

-- ============================================================
-- Tabela de controle de migrações
-- ============================================================
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(50) PRIMARY KEY,
    applied_at TIMESTAMPTZ DEFAULT NOW()
);
