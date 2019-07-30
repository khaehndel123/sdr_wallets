CREATE TABLE IF NOT EXISTS wallets
(
  id           TEXT PRIMARY KEY,
  address      TEXT                     NOT NULL DEFAULT '',
  generated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
  created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
  updated_at   TIMESTAMP WITH TIME ZONE,
  deleted_at   TIMESTAMP WITH TIME ZONE
);

CREATE UNIQUE INDEX IF NOT EXISTS uix_wallets_address ON wallets (address);

--

CREATE TABLE IF NOT EXISTS transactions
(
  id           TEXT PRIMARY KEY,
  block_number BIGINT,
  hash         TEXT UNIQUE              NOT NULL DEFAULT '',
  type         TEXT                     NOT NULL DEFAULT '',
  status       TEXT                     NOT NULL DEFAULT '',
  from_address TEXT,
  to_address   TEXT,
  value        TEXT,
  time         BIGINT,
  created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
  updated_at   TIMESTAMP WITH TIME ZONE,
  deleted_at   TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS ix_transactions_from ON transactions (from_address);
CREATE INDEX IF NOT EXISTS ix_transactions_to ON transactions (to_address);

--

CREATE TABLE IF NOT EXISTS transfers
(
  id              TEXT PRIMARY KEY,
  eth_transfer_id TEXT,
  transfer_type   TEXT                     NOT NULL DEFAULT '',
  from_address    TEXT                     NOT NULL DEFAULT '',
  nonce           BIGINT                   NOT NULL,
  gas_price       TEXT                     NOT NULL,
  gas_limit       BIGINT                   NOT NULL,
  status          TEXT                     NOT NULL DEFAULT '',
  tx_hash         TEXT,
  raw_tx          TEXT,
  message         TEXT                     NOT NULL DEFAULT '',
  created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
  updated_at      TIMESTAMP WITH TIME ZONE,
  deleted_at      TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS ix_transfers_eth_transfer ON transfers (eth_transfer_id);
CREATE INDEX IF NOT EXISTS ix_transfers_from ON transfers (from_address);
CREATE UNIQUE INDEX IF NOT EXISTS uix_transfers_hash ON transfers (tx_hash);
