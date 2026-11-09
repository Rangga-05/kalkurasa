-- Migration 003: Add lead_time_days and waste_pct per product
ALTER TABLE products ADD COLUMN IF NOT EXISTS lead_time_days INT NOT NULL DEFAULT 2;
ALTER TABLE products ADD COLUMN IF NOT EXISTS waste_pct NUMERIC(6,4) NOT NULL DEFAULT 0.05;

