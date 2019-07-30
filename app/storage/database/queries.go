package database

import "fmt"

var (
	queryTxHistoryBase = fmt.Sprintf(`
SELECT LOWER(COALESCE(tr.tx_hash, tr.id)) AS tx_hash,
       COALESCE(txout.status, tr.status) status,
       tr.amount,
       LOWER(tr.from_address) AS from_address,
       LOWER(tr.to_address) AS to_address,
       COALESCE(NULLIF(txout.time, 0), EXTRACT(epoch from txout.created_at)::bigint, EXTRACT(epoch from tr.created_at)::bigint) AS time,
       'out'        direction,
       COALESCE(txout.type, tr.transfer_type) as type
FROM transfers tr
       LEFT JOIN transactions txout ON tr.tx_hash = txout.hash
WHERE tr.transfer_type = '%s'
  AND LOWER(tr.from_address) = LOWER($1)
  AND tr.deleted_at IS NULL

UNION

SELECT LOWER(txin.hash) AS tx_hash,
       txin.status status,
       txin.value  amount,
       LOWER(txin.from_address) AS from_address,
       LOWER(txin.to_address) AS to_address,
       COALESCE(NULLIF(txin.time, 0), EXTRACT(epoch from txin.created_at)::bigint) AS time,
       'in'        direction,
       txin.type
FROM transactions txin
WHERE LOWER(txin.to_address) = LOWER($1)
  AND time >= $2
  AND LOWER(txin.from_address) <> LOWER($3)
  AND txin.type = '%s'
  AND txin.deleted_at IS NULL

UNION

SELECT LOWER(txout.hash) AS tx_hash,
       txout.status status,
       txout.value  amount,
       LOWER(txout.from_address) AS from_address,
       LOWER(txout.to_address) AS to_address,
       COALESCE(NULLIF(txout.time, 0), EXTRACT(epoch from txout.created_at)::bigint) AS time,
       'out'        direction,
       txout.type
FROM transactions txout
WHERE LOWER(txout.from_address) = LOWER($1)
  AND time >= $2
  AND LOWER(txout.to_address) <> LOWER($3)
  AND txout.type = '%s'
  AND txout.deleted_at IS NULL

ORDER BY time DESC, tx_hash DESC
`,
		TransferTypeTransferToken, TransferTypeTransferToken, TransferTypeTransferToken,
	)
	queryTxHistoryCount     = fmt.Sprintf("SELECT COUNT(*) FROM (%s) txes;", queryTxHistoryBase)
	queryTxHistoryPaginated = fmt.Sprintf("%s OFFSET $4 LIMIT $5;", queryTxHistoryBase)
)
