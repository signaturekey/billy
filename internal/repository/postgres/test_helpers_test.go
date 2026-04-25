package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/signaturekey/billy/internal/domain/entity"
)

func seedIntegrationUser(t *testing.T, pool *pgxpool.Pool, userID int64) {
	t.Helper()

	_, err := pool.Exec(
		context.Background(),
		`INSERT INTO users (id, email, password_hash)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		userID,
		fmt.Sprintf("user%d@example.test", userID),
		"test-hash",
	)
	require.NoError(t, err)
}

func createIntegrationAccount(
	t *testing.T,
	accounts *accountRepository,
	userID int64,
	currency string,
	balance int64,
	reservedAmount int64,
) entity.Account {
	t.Helper()

	seedIntegrationUser(t, accounts.pool, userID)

	account, err := accounts.Create(context.Background(), entity.Account{
		UserID:         userID,
		Currency:       currency,
		Balance:        balance,
		ReservedAmount: reservedAmount,
		Status:         entity.AccountStatusActive,
	})
	require.NoError(t, err)
	return account
}

func beginIntegrationTx(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()

	tx, err := pool.Begin(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = tx.Rollback(context.Background())
	})
	return tx
}

func newIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping PostgreSQL integration test in short mode")
	}

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set")
	}

	ctx := context.Background()
	schema := fmt.Sprintf("billy_test_%d", time.Now().UnixNano())
	require.True(t, isSafeSchemaName(schema), "unsafe test schema name")

	adminPool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)

	_, err = adminPool.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = make(map[string]string)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))

	applyIntegrationMigrations(t, pool)

	t.Cleanup(func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		adminPool.Close()
	})

	return pool
}

func applyIntegrationMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	migrations, err := filepath.Glob(filepath.Join(projectRoot(t), "migrations", "*.sql"))
	require.NoError(t, err)
	sort.Strings(migrations)
	require.NotEmpty(t, migrations)

	for _, migration := range migrations {
		body, err := os.ReadFile(migration)
		require.NoError(t, err)

		sql := upMigrationSQL(string(body))
		if strings.TrimSpace(sql) == "" {
			continue
		}
		execSimpleSQL(t, pool, sql)
	}
}

func execSimpleSQL(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Conn().PgConn().Exec(context.Background(), sql).ReadAll()
	require.NoError(t, err)
}

func projectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func upMigrationSQL(body string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	inDown := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "-- +goose Down"):
			inDown = true
			continue
		case strings.HasPrefix(trimmed, "-- +goose"):
			continue
		case inDown:
			continue
		default:
			out = append(out, line)
		}
	}

	return strings.Join(out, "\n")
}

func isSafeSchemaName(schema string) bool {
	if schema == "" {
		return false
	}

	for _, char := range schema {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '_' {
			continue
		}
		return false
	}

	return true
}
