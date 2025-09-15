# money design doc

# Core User Commands
- `money init`: initial setup command that prompts user for base64-encoded SimpleFIN setup token, exchanges it for Access URL, and stores credentials locally
  - Accepts base64-encoded setup tokens (not direct URLs)
  - No token validation - let SimpleFIN client handle errors
  - Works with both production and beta SimpleFIN bridges
- `money fetch`: syncs latest data from SimpleFIN and stores it to the local database
   - Uses stored Access URL from `money init` to fetch account data via GET /accounts endpoint
   - Data synced includes accounts and transactions with full history
   - Records balance snapshots for historical trending in balance command
   - Available Data Types:
     - Accounts: ID, name, currency, balance, available balance, balance date
     - Transactions: ID, posted timestamp, amount, description, pending status
     - Organizations: financial institution details
     - Custom currencies and exchange rates supported
   - Authentication: HTTPS with Basic Auth, SSL certificate verification required
- `money balance`: shows the current balance of all accounts + net worth with an ASCII graph showing balance trends over time grouped by account type (default last 30 days)
- `money accounts`: manage user accounts and account types
  - `money accounts list`: show all accounts with their current types and organizations
  - `money accounts type set <account-id> <type>`: set account type for better balance organization
    - Valid types: checking, savings, credit, investment, loan, other
  - `money accounts type clear <account-id>`: clear account type (set to unset)
  - `money accounts nickname set <account-id> <nickname>`: set a custom nickname for an account
  - `money accounts nickname clear <account-id>`: remove custom nickname (revert to original name)
- `money costs`: shows a breakdown of all costs by category for a given time period (default this month)
  - `--days|-d <number>`: show costs for the last N days (overrides other date options)
- `money income`: shows a breakdown of all income by category for a given time period (default this month)
  - `--days|-d <number>`: show income for the last N days (overrides other date options)
- `money transactions`: manage and view transactions 
    - `money transactions list [--start YYYY-MM-DD] [--end YYYY-MM-DD] [--account <account-id>] [--category <category-name>]`: list transactions with optional filtering by date range, account, or category
    - `money transactions categorize`: interactively categorize uncategorized transactions via llm.
        - transactions when fetched from simplefin are uncategorized
        - user can run this command to use a llm to categorize them
        - user can review and adjust categories as needed
        - `money transactions categorize auto [--all]`: automatically categorize transactions using LLM
        - `money transactions categorize manual`: fast spreadsheet-style TUI for manual transaction categorization
          - Vim-style keyboard navigation (j/k up/down, h/l left/right, gg/G top/bottom, Ctrl-f/Ctrl-b page up/down)
          - Quick category selection via numbered shortcuts for common categories
          - Bulk operations: multi-select transactions (v for visual mode), mark multiple as transfers
          - Inline category editing with auto-complete from existing categories
          - Progress tracking and color-coded transactions (expenses/income/transfers)
          - Vim-like modal interface with normal/insert/visual modes
        - `money transactions categorize modify <transaction-id> <category-name>`: manually set or change the category of a specific transaction
        - `money transactions categorize clear <transaction-id>`: clear the category of a specific transaction (set to uncategorized)
        - `money transactions categorize transfer <transaction-id>`: mark transaction as a transfer (excludes from income/expense calculations)
- `money transactions category`: manage transaction categories
  - `money transactions category list`: show all existing categories
  - `money transactions category add <name>`: add a new category
  - `money transactions category remove <name>`: remove a category (only if not used by any transactions)
  - `money transactions category seed`: populate database with common default categories

# Tech Stack
1. language: Go
2. cli library: github.com/rwxrob/bonzai v0.20.10:
   - Main executable in `cmd/money/main.go` that calls `cli.Cmd.Run()`
   - Root command defined in `cmd/money/cli/money.go` with Name, Summary, and Commands slice
   - Each subcommand (init, fetch, balance, costs, income, transactions) gets its own file in cli/ package
   - Commands structured as `&Z.Cmd{}` with Name, Summary, Call function, and optional sub-Commands
   - Use `Z "github.com/rwxrob/bonzai/z"` import alias pattern
3. storage: SQLite (local file-based database), dir for storage configured via the MONEY_DIR env var, defaults to $HOME/.money
4. ASCII graphing: github.com/guptarohit/asciigraph for balance trend visualization
5. TUI library: github.com/charmbracelet/bubbletea for interactive terminal interfaces
   - github.com/Evertras/bubble-table for spreadsheet-style transaction categorization
   - github.com/charmbracelet/lipgloss for styling and layout
6. LLM integration: For transaction categorization via `money categorize` command
   - API service for LLM calls (OpenAI/Anthropic/local model TBD)
   - Interactive prompting for category review and adjustment

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
    nickname TEXT,  -- Custom user-defined nickname for the account
    currency TEXT NOT NULL DEFAULT 'USD',
    balance INTEGER NOT NULL,  -- Store as cents to avoid floating point issues
    available_balance INTEGER,
    balance_date DATETIME,
    account_type TEXT CHECK (account_type IN ('checking', 'savings', 'credit', 'investment', 'loan', 'other', 'unset')) DEFAULT 'unset',
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
```
