package mallet

const (
	summaryStatement = "SELECT * FROM results ORDER BY started ASC;"
	oldRowsQuery     = "SELECT rowid FROM results WHERE (started >= ? AND started < ?);"
	deleteStatement  = "DELETE FROM results WHERE rowid IN (SELECT value FROM json_each(?));"
	minQuery         = "SELECT MIN(started) FROM results;"
	maxQuery         = "SELECT MAX(started) FROM results;"
	minRollupQuery   = "SELECT MIN(started) FROM results where rolled_up = false;"

	// API statements
	apiCreateStatement = `DROP TABLE IF EXISTS results;
		CREATE TABLE results(started INTEGER, latency INTEGER, min_latency INTEGER, p95 INTEGER, error INTEGER NOT NULL, rolled_up INTEGER DEFAULT 0, count INTEGER DEFAULT 1);
		CREATE INDEX index_started on results (started);`

	apiInsertStatement = "INSERT INTO results (started, latency, min_latency, p95, error) VALUES(?,?,?,?,?);"

	apiRollupQuery = `INSERT INTO results(
         started,
         latency, min_latency,
         p95,

         error,
         rolled_up,
         count
         )

         SELECT
         MIN(started) AS started,
         MAX(latency) AS latency,
         MIN(min_latency) AS min_latency,
         MAX(p95) as p95,

         SUM(error) AS error,
         MAX(1, 0) as rolled_up,
         SUM(count) AS count

         FROM results WHERE (started >= ? AND started < ?)
         ;`

	// NATS statements
	natsCreateStatement = `
		DROP TABLE IF EXISTS results;
		CREATE TABLE results(
			started INTEGER DEFAULT 0,

			latency INTEGER DEFAULT 0,
			min_latency INTEGER DEFAULT 0,
			p95 INTEGER DEFAULT 0,

			ack_pending INTEGER DEFAULT 0,
			min_ack_pending INTEGER DEFAULT 0,

			redelivered INTEGER DEFAULT 0,
			min_redelivered INTEGER DEFAULT 0,

			waiting INTEGER DEFAULT 0,
			min_waiting INTEGER DEFAULT 0,

			pending INTEGER DEFAULT 0,
			min_pending INTEGER DEFAULT 0,

			error INTEGER DEFAULT 0,
			rolled_up INTEGER DEFAULT 0,
			count INTEGER DEFAULT 1
		);
		CREATE INDEX index_started on results (started)
		`

	natsInsertStatement = `INSERT INTO results (
		started,
		latency, min_latency,

		p95,
		error,

		ack_pending, min_ack_pending,
		redelivered, min_redelivered,
		waiting, min_waiting,
		pending, min_pending



		)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?);`

	natsRollupQuery = `INSERT INTO results(
         started,
         latency, min_latency,
         p95,

         ack_pending, min_ack_pending,
         redelivered, min_redelivered,
         waiting, min_waiting,
         pending, min_pending,

         error,
         rolled_up,
         count
         )

         SELECT
         MIN(started) AS started,
         MAX(latency) AS latency,
         MIN(min_latency) AS min_latency,
         MAX(p95) as p95,

         MAX(ack_pending) AS ack_pending,
         MIN(min_ack_pending) AS min_ack_pending,

         MAX(redelivered) AS redelivered,
         MIN(min_redelivered) AS min_redelivered,

         MAX(waiting) AS waiting,
         MIN(min_waiting) AS min_waiting,

         MAX(pending) AS pending,
         MIN(min_pending) AS min_pending,

         SUM(error) AS error,
         MAX(1, 0) as rolled_up,
         SUM(count) AS count

         FROM results WHERE (started >= ? AND started < ?)
         ;`
)
