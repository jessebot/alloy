package collector

import (
	"database/sql/driver"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	loki_fake "github.com/grafana/alloy/internal/component/common/loki/client/fake"
	"github.com/grafana/alloy/internal/component/database_observability"
)

func TestQuerySample(t *testing.T) {
	defer goleak.VerifyNone(t)

	testcases := []struct {
		name       string
		rows       [][]driver.Value
		logsLabels []model.LabelSet
		logsLines  []string
	}{
		{
			name: "select query",
			rows: [][]driver.Value{{
				"some_schema",
				"some_digest",
				"select * from some_table where id = 1",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "truncated query",
			rows: [][]driver.Value{{
				"some_schema",
				"some_digest",
				"insert into some_table (`id1`, `id2`, `id3`, `id...",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}, {
				"some_schema",
				"some_digest",
				"select * from some_table where id = 1",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "truncated in multi-line comment",
			rows: [][]driver.Value{{
				"some_schema",
				"some_digest",
				"select * from some_table where id = 1 /*traceparent='00-abc...",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_PARSED_TABLE_NAME, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "truncated with properly closed comment",
			rows: [][]driver.Value{{
				"some_schema",
				"some_digest",
				"select * from some_table where id = 1 /* comment that's closed */ and name = 'test...",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{},
			logsLines:  []string{},
		},
		{
			name: "start transaction",
			rows: [][]driver.Value{{
				"some_schema",
				"some_digest",
				"START TRANSACTION",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="some_digest" digest_text="begin" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "sql parse error",
			rows: [][]driver.Value{{
				"some_schema",
				"some_digest",
				"select * from some_table where id = 1",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}, {
				"some_schema",
				"some_digest",
				"not valid sql",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "multiple schemas",
			rows: [][]driver.Value{{
				"some_schema",
				"some_digest",
				"select * from some_table where id = 1",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}, {
				"some_other_schema",
				"some_digest",
				"select * from some_table where id = 1",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
				`schema="some_other_schema" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "subquery and union",
			rows: [][]driver.Value{{
				"some_schema",
				"some_digest",
				"SELECT * FROM (SELECT id, name FROM employees_us_east UNION SELECT id, name FROM employees_us_west) as employees_us UNION SELECT id, name FROM employees_emea",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="some_digest" digest_text="select * from (select id, name from employees_us_east union select id, name from employees_us_west) as employees_us union select id, name from employees_emea" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
		{
			name: "show create table (table name is not parsed)",
			rows: [][]driver.Value{{
				"some_schema",
				"some_digest",
				"SHOW CREATE TABLE some_table",
				"70000000",
				"20000000",
				"10000000",
				"5",
				"5",
				"0",
				"0",
				"456",
				"457",
			}},
			logsLabels: []model.LabelSet{
				{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"},
			},
			logsLines: []string{
				`schema="some_schema" digest="some_digest" digest_text="show create table" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`,
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			lokiClient := loki_fake.NewClient(func() {})

			collector, err := NewQuerySample(QuerySampleArguments{
				DB:              db,
				InstanceKey:     "mysql-db",
				CollectInterval: time.Second,
				EntryHandler:    lokiClient,
				Logger:          log.NewLogfmtLogger(os.Stderr),
			})
			require.NoError(t, err)
			require.NotNil(t, collector)

			mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"uptime",
					}).AddRow(
						"1", // corresponds to initial timerBookmark
					),
				)

			mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
				sqlmock.NewRows([]string{
					"now",
					"uptime",
				}).AddRow(
					1,
					2,
				),
			)

			mock.ExpectQuery(selectQuerySamples+endOfTimeline).WithArgs(
				1e12, // initial timerBookmark
				2e12,
			).RowsWillBeClosed().
				WillReturnRows(
					sqlmock.NewRows([]string{
						"statements.CURRENT_SCHEMA",
						"statements.DIGEST",
						"statements.DIGEST_TEXT",
						"statements.TIMER_END",
						"statements.TIMER_WAIT",
						"statements.CPU_TIME",
						"statements.ROWS_EXAMINED",
						"statements.ROWS_SENT",
						"statements.ROWS_AFFECTED",
						"statements.ERRORS",
						"statements.MAX_CONTROLLED_MEMORY",
						"statements.MAX_TOTAL_MEMORY",
					}).AddRows(
						tc.rows...,
					),
				)

			err = collector.Start(t.Context())
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				return len(lokiClient.Received()) == len(tc.logsLines)
			}, 5*time.Second, 100*time.Millisecond)

			collector.Stop()
			lokiClient.Stop()

			require.Eventually(t, func() bool {
				return collector.Stopped()
			}, 5*time.Second, 100*time.Millisecond)

			err = mock.ExpectationsWereMet()
			require.NoError(t, err)

			lokiEntries := lokiClient.Received()
			require.Equal(t, len(tc.logsLines), len(lokiEntries))
			for i, entry := range lokiEntries {
				require.Equal(t, tc.logsLabels[i], entry.Labels)
				require.Equal(t, tc.logsLines[i], entry.Line)
			}
		})
	}
}

func TestQuerySampleSQLDriverErrors(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("recoverable sql error in result set", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"uptime",
				}).AddRow(
					"1",
				))

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				1,
				2,
			))

		mock.ExpectQuery(selectQuerySamples+endOfTimeline).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"digest", // not enough columns
				}).AddRow(
					"abc123",
				))

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				1,
				2,
			))

		mock.ExpectQuery(selectQuerySamples+endOfTimeline).WithArgs(
			2e12, // initial timerBookmark
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"some_digest",
					"select * from some_table where id = 1",
					"70000000",
					"20000000",
					"10000000",
					"5",
					"5",
					"0",
					"0",
					"456",
					"457",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `schema="some_schema" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`, lokiEntries[0].Line)
	})

	t.Run("result set iteration error", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"uptime",
				}).AddRow(
					"1",
				))

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				1,
				2,
			))

		mock.ExpectQuery(selectQuerySamples+endOfTimeline).WithArgs(
			1e12, // initial timerBookmark
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"some_digest",
					"select * from some_table where id = 1",
					"70000000",
					"20000000",
					"10000000",
					"5",
					"5",
					"0",
					"0",
					"456",
					"457",
				).AddRow(
					"some_schema",
					"some_digest",
					"select * from some_table where id = 1",
					"70000000",
					"20000000",
					"10000000",
					"5",
					"5",
					"0",
					"0",
					"456",
					"457",
				).RowError(1, fmt.Errorf("rs error")), // error on second row
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `schema="some_schema" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`, lokiEntries[0].Line)
	})

	t.Run("connection error recovery", func(t *testing.T) {
		t.Parallel()

		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := loki_fake.NewClient(func() {})

		collector, err := NewQuerySample(QuerySampleArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(selectUptime).WithoutArgs().RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"uptime",
				}).AddRow(
					"1",
				))

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				1,
				2,
			))

		mock.ExpectQuery(selectQuerySamples+endOfTimeline).WithArgs(
			1e12,
			2e12,
		).WillReturnError(fmt.Errorf("connection error"))

		mock.ExpectQuery(selectNowAndUptime).WithoutArgs().WillReturnRows(
			sqlmock.NewRows([]string{
				"now",
				"uptime",
			}).AddRow(
				1,
				2,
			))

		mock.ExpectQuery(selectQuerySamples+endOfTimeline).WithArgs(
			1e12,
			2e12,
		).RowsWillBeClosed().
			WillReturnRows(
				sqlmock.NewRows([]string{
					"statements.CURRENT_SCHEMA",
					"statements.DIGEST",
					"statements.DIGEST_TEXT",
					"statements.TIMER_END",
					"statements.TIMER_WAIT",
					"statements.CPU_TIME",
					"statements.ROWS_EXAMINED",
					"statements.ROWS_SENT",
					"statements.ROWS_AFFECTED",
					"statements.ERRORS",
					"statements.MAX_CONTROLLED_MEMORY",
					"statements.MAX_TOTAL_MEMORY",
				}).AddRow(
					"some_schema",
					"some_digest",
					"select * from some_table where id = 1",
					"70000000",
					"20000000",
					"10000000",
					"5",
					"5",
					"0",
					"0",
					"456",
					"457",
				),
			)

		err = collector.Start(t.Context())
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return len(lokiClient.Received()) == 1
		}, 5*time.Second, 100*time.Millisecond)

		collector.Stop()
		lokiClient.Stop()

		require.Eventually(t, func() bool {
			return collector.Stopped()
		}, 5*time.Second, 100*time.Millisecond)

		err = mock.ExpectationsWereMet()
		require.NoError(t, err)

		lokiEntries := lokiClient.Received()
		require.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		require.Equal(t, `schema="some_schema" digest="some_digest" digest_text="select * from some_table where id = :redacted1" rows_examined="5" rows_sent="5" rows_affected="0" errors="0" max_controlled_memory="456b" max_total_memory="457b" cpu_time="0.010000ms" elapsed_time="0.020000ms" elapsed_time_ms="0.020000ms"`, lokiEntries[0].Line)
	})
}

func TestQuerySampleSummary_initializeTimer(t *testing.T) {
	t.Run("selects uptime, sets timerBookmark", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{
			"uptime",
		}).AddRow(
			5,
		))

		c := &QuerySample{dbConnection: db}
		require.NoError(t, c.initializeBookmark(t.Context()))

		assert.Equal(t, 5e12, c.timerBookmark)
	})

	t.Run("sets timerBookmark as uptime modulo overflows (uptime is comprised of 1 overflow)", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectQuery(selectUptime).WithoutArgs().WillReturnRows(sqlmock.NewRows([]string{
			"uptime",
		}).AddRow(
			picosecondsToSeconds(math.MaxUint64) + 5,
		))

		c := &QuerySample{dbConnection: db}
		require.NoError(t, c.initializeBookmark(t.Context()))

		assert.Equal(t, 5e12, c.timerBookmark)
	})
}
