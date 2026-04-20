package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	rrdomain "example.com/taskservice/internal/domain/recurrencerule"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const insertTaskQuery = `
        INSERT INTO tasks (title, description, status, is_recurring, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id
    `

	const insertRecurrenceRuleQuery = `
        INSERT INTO recurrence_rules (
            task_id,
            is_everyday,
            parity,
            days_of_week,
            days_of_month,
            specific_dates,
            next_run_at,
            is_expired,
            start_rule_date,
            end_rule_date
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var taskID int64

	err = tx.QueryRow(
		ctx,
		insertTaskQuery,
		task.Title,
		task.Description,
		task.Status,
		task.IsRecurring,
		task.CreatedAt,
		task.UpdatedAt,
	).Scan(&taskID)
	if err != nil {
		return nil, err
	}

	if task.IsRecurring && !task.RecurrenceRule.Empty() {
		_, err = tx.Exec(
			ctx,
			insertRecurrenceRuleQuery,
			taskID,
			task.RecurrenceRule.IsEveryday,
			task.RecurrenceRule.Parity,
			task.RecurrenceRule.DaysOfWeek,
			task.RecurrenceRule.DaysOfMonth,
			task.RecurrenceRule.SpecificDates,
			task.RecurrenceRule.NextRunAt,
			task.RecurrenceRule.IsExpired,
			task.RecurrenceRule.StartRuleDate,
			task.RecurrenceRule.EndRuleDate,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	task.ID = taskID

	if _, err = updateIfDue(ctx, r.pool, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT
            t.id,
            t.title,
            t.description,
            t.status,
            t.is_recurring,
            t.created_at,
            t.updated_at,
            r.is_everyday,
            r.parity,
            r.days_of_week,
            r.days_of_month,
            r.specific_dates,
            r.next_run_at,
            r.is_expired
            r.start_rule_date,
            r.end_rule_date
        FROM tasks t
        LEFT JOIN recurrence_rules r ON r.task_id = t.id
        WHERE t.id = $1;
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	if _, err = updateIfDue(ctx, r.pool, found); err != nil {
		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const updateTaskQuery = `
		UPDATE tasks
		SET title = $1,
			description = $2,
			status = $3,
			is_recurring = $4,
			updated_at = $5
		WHERE id = $6
	`

	const upsertRecurrenceRuleQuery = `
		INSERT INTO recurrence_rules (
			task_id,
            is_everyday,
			parity,
			days_of_week,
			days_of_month,
            specific_dates,
            next_run_at,
			is_expired,
			start_rule_date,
			end_rule_date
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (task_id) DO UPDATE SET
            is_everyday = EXCLUDED.is_everyday,
			parity = EXCLUDED.parity,
			days_of_week = EXCLUDED.days_of_week,
			days_of_month = EXCLUDED.days_of_month,
			specific_dates = EXCLUDED.specific_dates,
            next_run_at = EXCLUDED.next_run_at,
            is_expired = EXCLUDED.is_expired,
			start_rule_date = EXCLUDED.start_rule_date,
			end_rule_date = EXCLUDED.end_rule_date
	`

	const deleteRecurrenceRuleQuery = `
		DELETE FROM recurrence_rules
		WHERE task_id = $1
	`

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(
		ctx,
		updateTaskQuery,
		task.Title,
		task.Description,
		task.Status,
		task.IsRecurring,
		task.UpdatedAt,
		task.ID,
	)
	if err != nil {
		return nil, err
	}

	if task.IsRecurring && !task.RecurrenceRule.Empty() {
		_, err = tx.Exec(
			ctx,
			upsertRecurrenceRuleQuery,
			task.ID,
			task.RecurrenceRule.IsEveryday,
			task.RecurrenceRule.Parity,
			task.RecurrenceRule.DaysOfWeek,
			task.RecurrenceRule.DaysOfMonth,
			task.RecurrenceRule.SpecificDates,
			task.RecurrenceRule.NextRunAt,
			task.RecurrenceRule.IsExpired,
			task.RecurrenceRule.StartRuleDate,
			task.RecurrenceRule.EndRuleDate,
		)
		if err != nil {
			return nil, err
		}
	} else {
		_, err = tx.Exec(ctx, deleteRecurrenceRuleQuery, task.ID)
		if err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	if _, err = updateIfDue(ctx, r.pool, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT
			t.id,
			t.title,
			t.description,
			t.status,
			t.is_recurring,
			t.created_at,
			t.updated_at,
            r.is_everyday,
			r.parity,
			r.days_of_week,
			r.days_of_month,
			r.specific_dates,
            r.next_run_at,
            r.is_expired,
			r.start_rule_date,
			r.end_rule_date
		FROM tasks t
		LEFT JOIN recurrence_rules r ON r.task_id = t.id
		ORDER BY t.id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func(tx pgx.Tx, ctx context.Context) {
		_ = tx.Rollback(ctx)
	}(tx, ctx)

	for _, task := range tasks {
		if _, err = updateIfDue(ctx, tx, &task); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return tasks, nil
}

// utility

type DBExec interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func updateIfDue(ctx context.Context, db DBExec, task *taskdomain.Task) (bool, error) {
	const updateExpiredQuery = `
        UPDATE recurrence_rules 
        SET is_expired = $1 
        WHERE id = $2;
    `

	const updateNextRunAtQuery = `
		UPDATE recurrence_rules
		SET next_run_at = $1
		WHERE task_id = $2
	`

	if task.RecurrenceRule.Empty() ||
		task.RecurrenceRule.IsExpired ||
		time.Now().UTC().Before(task.RecurrenceRule.NextRunAt) {
		return false, nil
	}

	nextRunDate, active := rrdomain.CalcNextRunAt(&task.RecurrenceRule, time.Now().UTC())

	if !active {
		tag, err := db.Exec(ctx, updateExpiredQuery, !active, task.ID)
		if err != nil {
			return false, err
		}
		task.RecurrenceRule.IsExpired = true
		return tag.RowsAffected() > 0, nil
	}

	tag, err := db.Exec(ctx, updateNextRunAtQuery, nextRunDate, task.ID)
	if err != nil {
		return false, err
	}

	task.RecurrenceRule.NextRunAt = nextRunDate
	task.RecurrenceRule.IsExpired = false

	fmt.Println(task)

	return tag.RowsAffected() > 0, nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task        taskdomain.Task
		status      string
		parity      int32
		endRuleDate pgtype.Timestamptz
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&task.IsRecurring,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.RecurrenceRule.IsEveryday,
		&parity,
		&task.RecurrenceRule.DaysOfWeek,
		&task.RecurrenceRule.DaysOfMonth,
		&task.RecurrenceRule.SpecificDates,
		&task.RecurrenceRule.NextRunAt,
		&task.RecurrenceRule.IsExpired,
		&task.RecurrenceRule.StartRuleDate,
		&endRuleDate,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)
	task.RecurrenceRule.Parity = rrdomain.Parity(parity)

	// cause endRuleDate may be NULL
	if endRuleDate.Valid {
		task.RecurrenceRule.EndRuleDate = endRuleDate.Time
	} else {
		task.RecurrenceRule.EndRuleDate = time.Time{}
	}

	return &task, nil
}
