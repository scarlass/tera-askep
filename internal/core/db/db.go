package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/scarlass/tera-askep/internal/core"
	"github.com/scarlass/tera-askep/internal/core/configs"
	"github.com/scarlass/tera-askep/internal/core/utils"
)

func to_dsn(conf configs.DatabaseConfig) (*pgxpool.Config, error) {
	params := []string{}

	param := func(k string, v any) string {
		return fmt.Sprintf("%s=%v", k, v)
	}

	params = append(params, param("host", conf.Host))
	params = append(params, param("port", conf.Port))
	params = append(params, param("dbname", conf.Database))
	params = append(params, param("user", conf.User))
	params = append(params, param("password", conf.Password))
	params = append(params, param("search_path", conf.Schema))

	return pgxpool.ParseConfig(strings.Join(params, " "))
}

type Database struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, conf configs.DatabaseConfig) (*Database, error) {
	pgxconf, err := to_dsn(conf)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, pgxconf)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return &Database{pool}, nil
}

func (db *Database) UpdateAskepList(ctx context.Context, target configs.TargetConfig, b64Content string) (err error) {

	var tx pgx.Tx

	defer func() {
		if rec := recover(); rec != nil {
			switch v := rec.(type) {
			case error:
				err = v
			case string:
				err = errors.New(v)
			default:
				err = fmt.Errorf("%v", v)
			}
		}

		if tx != nil {
			if err != nil {
				tx.Rollback(ctx)
			} else {
				err = tx.Commit(ctx)
			}
		}
	}()

	tx, err = db.pool.Begin(ctx)
	if err != nil {
		return err
	}

	sql := `UPDATE askep_list
        SET form_data = convert_from(decode('{{ .content }}', 'base64'), 'UTF8')
	WHERE alid = {{ .alid }};`
	sql = utils.Must(core.ReplaceTemplateString(sql,
		map[string]any{
			"alid":    target.Alid,
			"content": b64Content,
		}))

	_, err = tx.Exec(ctx, sql)
	return
}
