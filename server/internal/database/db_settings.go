package database

import "context"

// ========== Settings ==========

func (db *DB) GetAllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			result[k] = v
		}
	}
	return result, nil
}

func (db *DB) SetSetting(ctx context.Context, key, value string) error {
	_, err := db.conn.ExecContext(ctx, 
		`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
		key, value,
	)
	return err
}

func (db *DB) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := db.conn.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&value)
	return value, err
}
