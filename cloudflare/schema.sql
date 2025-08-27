-- Cracked Dev-o-Meter Database Schema
-- Run this with: wrangler d1 execute cracked-dev-o-meter-users --file=schema.sql
-- Users table
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT,
    ip_address TEXT NOT NULL,
    user_agent TEXT,
    is_paid BOOLEAN DEFAULT FALSE,
    stripe_customer_id TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
-- Request logs table
CREATE TABLE IF NOT EXISTS request_logs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    endpoint TEXT NOT NULL,
    method TEXT NOT NULL,
    user_agent TEXT,
    created_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
-- Payments table
CREATE TABLE IF NOT EXISTS payments (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    stripe_payment_id TEXT NOT NULL,
    amount INTEGER NOT NULL,
    currency TEXT NOT NULL,
    status TEXT NOT NULL,
    type TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);
-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_ip ON users(ip_address);
CREATE INDEX IF NOT EXISTS idx_request_logs_user_id ON request_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_request_logs_created_at ON request_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);