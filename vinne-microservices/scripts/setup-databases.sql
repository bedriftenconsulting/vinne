-- Setup databases and users for all microservices
-- Run this on the PostgreSQL primary server

-- Admin Management Service
CREATE DATABASE admin_management;
CREATE USER admin_mgmt WITH PASSWORD 'AdminMgmt2025Secure';
GRANT ALL PRIVILEGES ON DATABASE admin_management TO admin_mgmt;
ALTER DATABASE admin_management OWNER TO admin_mgmt;

-- Agent Auth Service
CREATE DATABASE agent_auth;
CREATE USER agent WITH PASSWORD 'AgentAuth2025Secure';
GRANT ALL PRIVILEGES ON DATABASE agent_auth TO agent;
ALTER DATABASE agent_auth OWNER TO agent;

-- Agent Management Service
CREATE DATABASE agent_management;
CREATE USER agent_mgmt WITH PASSWORD 'AgentMgmt2025Secure';
GRANT ALL PRIVILEGES ON DATABASE agent_management TO agent_mgmt;
ALTER DATABASE agent_management OWNER TO agent_mgmt;

-- Draw Service
CREATE DATABASE draw_service;
CREATE USER draw WITH PASSWORD 'Draw2025Secure';
GRANT ALL PRIVILEGES ON DATABASE draw_service TO draw;
ALTER DATABASE draw_service OWNER TO draw;

-- Game Service
CREATE DATABASE game_service;
CREATE USER game WITH PASSWORD 'Game2025Secure';
GRANT ALL PRIVILEGES ON DATABASE game_service TO game;
ALTER DATABASE game_service OWNER TO game;

-- Payment Service
CREATE DATABASE payment_service;
CREATE USER payment WITH PASSWORD 'Payment2025Secure';
GRANT ALL PRIVILEGES ON DATABASE payment_service TO payment;
ALTER DATABASE payment_service OWNER TO payment;

-- Terminal Service
CREATE DATABASE terminal_service;
CREATE USER terminal WITH PASSWORD 'Terminal2025Secure';
GRANT ALL PRIVILEGES ON DATABASE terminal_service TO terminal;
ALTER DATABASE terminal_service OWNER TO terminal;

-- Wallet Service
CREATE DATABASE wallet_service;
CREATE USER wallet WITH PASSWORD 'Wallet2025Secure';
GRANT ALL PRIVILEGES ON DATABASE wallet_service TO wallet;
ALTER DATABASE wallet_service OWNER TO wallet;

-- Grant schema permissions after connecting to each database
\c admin_management
GRANT ALL ON SCHEMA public TO admin_mgmt;

\c agent_auth
GRANT ALL ON SCHEMA public TO agent;

\c agent_management
GRANT ALL ON SCHEMA public TO agent_mgmt;

\c draw_service
GRANT ALL ON SCHEMA public TO draw;

\c game_service
GRANT ALL ON SCHEMA public TO game;

\c payment_service
GRANT ALL ON SCHEMA public TO payment;

\c terminal_service
GRANT ALL ON SCHEMA public TO terminal;

\c wallet_service
GRANT ALL ON SCHEMA public TO wallet;