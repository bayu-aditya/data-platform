-- Baseline migration. It creates nothing; it exists so the embed → goose →
-- goose_db_version pipeline is exercised end to end before the first real
-- table lands. Domain tables arrive in later migrations.

-- +goose Up

-- +goose Down
