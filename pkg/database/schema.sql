-- Store SimpleFIN credentials
CREATE TABLE credentials (
    id INTEGER PRIMARY KEY,
    access_url TEXT NOT NULL,
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_used DATETIME
);

-- Financial institutions/organizations
CREATE TABLE organizations (
    id TEXT PRIMARY KEY,  -- SimpleFIN org ID
    name TEXT NOT NULL,
    url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- User accounts
CREATE TABLE accounts (
    id TEXT PRIMARY KEY,  -- SimpleFIN account ID
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    currency TEXT NOT NULL DEFAULT 'USD',
    balance INTEGER NOT NULL,  -- Store as cents to avoid floating point issues
    available_balance INTEGER,
    balance_date DATETIME,
    account_type TEXT CHECK (account_type IN ('checking', 'savings', 'credit', 'investment', 'loan', 'other')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_id) REFERENCES organizations(id)
);

-- Categories for transaction classification
CREATE TABLE categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Balance history for trending
CREATE TABLE balance_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id TEXT NOT NULL,
    balance INTEGER NOT NULL,  -- Store as cents
    available_balance INTEGER,
    recorded_at DATETIME NOT NULL,
    FOREIGN KEY (account_id) REFERENCES accounts(id)
);

-- Transactions
CREATE TABLE transactions (
    id TEXT PRIMARY KEY,  -- SimpleFIN transaction ID
    account_id TEXT NOT NULL,
    posted DATETIME NOT NULL,
    amount INTEGER NOT NULL,  -- Store as cents
    description TEXT NOT NULL,
    pending BOOLEAN DEFAULT FALSE,
    is_transfer BOOLEAN DEFAULT FALSE,  -- Excludes from income/expense calculations
    category_id INTEGER,  -- NULL for uncategorized transactions
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id),
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Indexes for performance
CREATE INDEX idx_transactions_account_id ON transactions(account_id);
CREATE INDEX idx_transactions_posted ON transactions(posted);
CREATE INDEX idx_transactions_category_id ON transactions(category_id);
CREATE INDEX idx_transactions_is_transfer ON transactions(is_transfer);
CREATE INDEX idx_accounts_org_id ON accounts(org_id);
CREATE INDEX idx_balance_history_account_id ON balance_history(account_id);
CREATE INDEX idx_balance_history_recorded_at ON balance_history(recorded_at);