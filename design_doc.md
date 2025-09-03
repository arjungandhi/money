# money design doc

# Core User Commands
- `money init`: initial setup command that prompts user to get SimpleFIN token from SimpleFIN Bridge, exchanges it for Access URL, and stores credentials locally
- `money fetch`: syncs latest data from SimpleFIN and stores it to the local database
   - Uses stored Access URL from `money init` to fetch account data via GET /accounts endpoint
   - Data synced includes accounts and transactions with full history
   - Available Data Types:
     - Accounts: ID, name, currency, balance, available balance, balance date
     - Transactions: ID, posted timestamp, amount, description, pending status
     - Organizations: financial institution details
     - Custom currencies and exchange rates supported
   - Authentication: HTTPS with Basic Auth, SSL certificate verification required
- `money balance`: shows the current balance of all accounts + net worth
- `money costs`: shows a breakdown of all costs by category for a given time period (default this month)
- `money income`: shows a breakdown of all income by category for a given time period (default this month)

# Tech Stack
1. language: Go
2. cli library: github.com/rwxrob/bonzai v0.20.10:
   - Main executable in `cmd/money/main.go` that calls `cli.Cmd.Run()`
   - Root command defined in `cmd/money/cli/money.go` with Name, Summary, and Commands slice
   - Each subcommand (init, fetch, balance, costs, income) gets its own file in cli/ package
   - Commands structured as `&Z.Cmd{}` with Name, Summary, Call function, and optional sub-Commands
   - Use `Z "github.com/rwxrob/bonzai/z"` import alias pattern
3. storage: SQLite (local file-based database), dir for storage configured via the MONEY_DIR env var, defaults to $HOME/.money

# Database Schema

```sql
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
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_id) REFERENCES organizations(id)
);

-- Transactions
CREATE TABLE transactions (
    id TEXT PRIMARY KEY,  -- SimpleFIN transaction ID
    account_id TEXT NOT NULL,
    posted DATETIME NOT NULL,
    amount INTEGER NOT NULL,  -- Store as cents
    description TEXT NOT NULL,
    pending BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id)
);

-- Indexes for performance
CREATE INDEX idx_transactions_account_id ON transactions(account_id);
CREATE INDEX idx_transactions_posted ON transactions(posted);
CREATE INDEX idx_accounts_org_id ON accounts(org_id);
```
