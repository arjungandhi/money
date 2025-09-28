# Money CLI

A powerful personal finance management CLI tool built in Go that connects to your bank accounts via SimpleFIN and provides comprehensive financial tracking, budgeting, and transaction categorization.

[![Go Reference](https://pkg.go.dev/badge/github.com/:user/:repo.svg)](https://pkg.go.dev/github.com/:user/:repo)
[![Go Report Card](https://goreportcard.com/badge/gojp/goreportcard)](https://goreportcard.com/report/:user/:repo) 
[![License](https://img.shields.io/github/license/:user/:repo)](https://github.com/:user/:repo/blob/main/LICENSE)


## Features
- 🏦 **Bank Integration**: Connect to your accounts via SimpleFIN for automatic transaction sync
- 📊 **Balance Tracking**: View current balances and net worth with ASCII trend graphs
- 🏷️ **Smart Categorization**: Automatic transaction categorization using LLM integration
- 💰 **Budgeting**: Comprehensive budget views with income/expense breakdown by category
- 🏠 **Property Management**: Track real estate values using RentCast API integration
- 📱 **Interactive TUI**: Vim-style interface for manual transaction categorization
- 🔒 **Local Storage**: All data stored locally in SQLite - your financial data stays private

## Setup

1. Create an account on [SimpleFIN](https://beta-bridge.simplefin.com), connect your bank accounts, and get your setup token.
   a. PS. you can get a demo token from [here](https://beta-bridge.simplefin.org/info/developers).
3. Obtain a RentCast API key from [RentCast](https://www.rentcast.io).

1. **Install**
    a. Latest Release (single binary):
    ```bash
    # Linux (amd64)
    wget https://github.com/arjungandhi/money/releases/latest/download/money-linux-amd64 -O money
    chmod +x money
    sudo mv money /usr/local/bin/

    # Linux (arm64)
    wget https://github.com/arjungandhi/money/releases/latest/download/money-linux-arm64 -O money
    chmod +x money
    sudo mv money /usr/local/bin/

    # macOS (amd64)
    wget https://github.com/arjungandhi/money/releases/latest/download/money-darwin-amd64 -O money
    chmod +x money
    sudo mv money /usr/local/bin/

    # macOS (arm64/Apple Silicon)
    wget https://github.com/arjungandhi/money/releases/latest/download/money-darwin-arm64 -O money
    chmod +x money
    sudo mv money /usr/local/bin/
    ```
    b. From Source:
    ```bash
    go install github.com/arjungandhi/money/cmd/money@latest
    ```

2. **Initialize** with guided setup:
```bash
money init
```

3. **Fetch** your latest transactions:
```bash
money fetch
```

4. **View** your balance and trends:
```bash
money balance
```

## Core Commands

- `money init` - Interactive setup for SimpleFIN, RentCast, and LLM integration
- `money fetch` - Sync latest transactions from your bank accounts
- `money balance` - Show current balances with trend visualization
- `money budget` - View income/expense breakdown by category
- `money transactions` - Manage and categorize transactions (TUI or CLI)
- `money accounts` - Manage account types and nicknames
- `money categories` - Manage transaction categories
- `money property` - Track real estate values and manage properties
- `money version` - Display the current version of the money CLI
- `money update` - Update the money CLI to the latest version from GitHub releases

# Design Doc
The design doc can be found [here](docs/design.md).

made by [a monkey](www.arjungandhi.com)
