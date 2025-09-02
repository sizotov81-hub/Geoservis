package adapter

import (
	"context"
	"database/sql"
	"fmt"

	"reflect"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
)

type SQLAdapter struct {
	DB *sqlx.DB
}

func NewSQLAdapter(db *sqlx.DB) *SQLAdapter {
	return &SQLAdapter{DB: db}
}

/*func (a *SQLAdapter) Create(ctx context.Context, entity interface{}, tableName string) error {
	data := toMap(entity)
	if len(data) == 0 {
		return fmt.Errorf("no data to insert")
	}

	query, args, err := sq.Insert(tableName).SetMap(data).ToSql()
	if err != nil {
		return fmt.Errorf("failed to build insert query: %w", err)
	}

	_, err = a.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert record: %w", err)
	}

	return nil
}*/

func (a *SQLAdapter) Create(ctx context.Context, entity interface{}, tableName string) error {
	data := toMap(entity)
	if len(data) == 0 {
		return fmt.Errorf("no data to insert")
	}

	query, args, err := sq.Insert(tableName).SetMap(data).ToSql()
	if err != nil {
		return fmt.Errorf("failed to build insert query: %w", err)
	}

	// Логирование для отладки
	fmt.Printf("SQL Query: %s\nArgs: %v\n", query, args)

	_, err = a.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to insert record: %w", err)
	}

	return nil
}

func (a *SQLAdapter) Update(ctx context.Context, entity interface{}, tableName string, condition Condition) error {
	query, args, err := sq.Update(tableName).
		SetMap(toMap(entity)).
		Where(condition.Equal).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build update query: %w", err)
	}

	_, err = a.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}

	return nil
}

func (a *SQLAdapter) Delete(ctx context.Context, tableName string, condition Condition) error {
	query, args, err := sq.Update(tableName).
		Set("deleted_at", sq.Expr("NOW()")).
		Where(condition.Equal).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build delete query: %w", err)
	}

	_, err = a.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to soft delete record: %w", err)
	}

	return nil
}

func (a *SQLAdapter) List(ctx context.Context, dest interface{}, tableName string, condition Condition) error {
	query, args, err := sq.Select("*").
		From(tableName).
		Where(condition.Equal).
		Where(sq.Eq{"deleted_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build select query: %w", err)
	}

	err = a.DB.SelectContext(ctx, dest, query, args...)
	if err != nil {
		return fmt.Errorf("failed to select records: %w", err)
	}

	return nil
}

func (a *SQLAdapter) Get(ctx context.Context, dest interface{}, tableName string, condition Condition) error {
	query, args, err := sq.Select("*").
		From(tableName).
		Where(condition.Equal).
		Where(sq.Eq{"deleted_at": nil}).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build select query: %w", err)
	}

	err = a.DB.GetContext(ctx, dest, query, args...)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("record not found")
		}
		return fmt.Errorf("failed to get record: %w", err)
	}

	return nil
}

func toMap(entity interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	val := reflect.ValueOf(entity)

	// Если передали указатель, берем значение
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Проверяем, что это структура
	if val.Kind() != reflect.Struct {
		return result
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		// Пропускаем неэкспортируемые поля
		if !field.CanInterface() {
			continue
		}

		fieldType := typ.Field(i)
		tag := fieldType.Tag.Get("db")
		if tag == "" || tag == "-" {
			continue
		}

		// Удаляем опции тега (например, ",omitempty")
		if commaIdx := strings.Index(tag, ","); commaIdx != -1 {
			tag = tag[:commaIdx]
		}

		// Обрабатываем только не-нулевые значения
		if !field.IsZero() {
			result[tag] = field.Interface()
		}
	}
	return result
}

type Condition struct {
	Equal sq.Eq
}
