CREATE TABLE IF NOT EXISTS tasks (
	id BIGSERIAL PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
    is_recurring BOOLEAN NOT NULL DEFAULT FALSE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS recurrence_rules (
    task_id BIGINT PRIMARY KEY,
    is_everyday BOOLEAN NOT NULL,
    parity INTEGER NOT NULL,
    days_of_week INTEGER[],
    days_of_month INTEGER[],
    specific_dates TIMESTAMPTZ[],
    next_run_at TIMESTAMPTZ NOT NULL,
    is_expired BOOLEAN NOT NULL,
    start_rule_date TIMESTAMPTZ NOT NULL,
    end_rule_date TIMESTAMPTZ,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks (status);
