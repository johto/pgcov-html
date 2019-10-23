package main

import (
	"github.com/lib/pq"
	"database/sql"
)

type sourceLine struct {
	lineno int32
	ncalls int32
}

type Function struct {
	Signature string
	Calls int32
	Coverage float64
	prosrc *string
	sourceLines []sourceLine
}

type Fetcher struct {
	db *sql.DB
	txn *sql.Tx
	pid int32
}

func (f *Fetcher) Listen(conninfo string) (notify chan error, err error) {
	var db *sql.DB
	var txn *sql.Tx

	conninfo = "fallback_application_name=pgcov-html " + conninfo
	db, err = sql.Open("postgres", conninfo)
	if err != nil {
		return nil, err
	}

	txn, err = db.Begin()
	if err != nil {
		return nil, err
	}

	// kill any previous listeners XXX fix for 9.2+
	_, err = txn.Exec("SELECT pg_cancel_backend(pid) FROM pg_stat_activity WHERE application_name='pgcov-html' AND pid <> pg_backend_pid()")
	if err != nil {
		_ = txn.Rollback()
		return nil, err
	}

	err = txn.QueryRow("SELECT pg_backend_pid()").Scan(&f.pid)
	if err != nil {
		_ = txn.Rollback()
		return nil, err
	}

	// We have to do pg_cancel_backend() below, which aborts the transaction
	// we're in.  However, we also need to hold on to the transaction because
	// that used to be the only way to hold on to the connection, which is what
	// we really want to do.  I'm not going to bother changing this code to use
	// the new *sql.Conn stuff, because I'm lazy.
	_, err = txn.Exec("SAVEPOINT s")
	if err != nil {
		_ = txn.Rollback()
		return nil, err
	}

	f.db = db
	f.txn = txn
	notify = make(chan error, 1)
	go f.listener(notify)
	return notify, nil
}

func (f *Fetcher) Done(notify chan error) (functions []*Function, err error) {
	defer func() {
		_ = f.txn.Rollback()
		f.txn = nil
		f.db.Close()
		f.db = nil
	}()

	_, err = f.db.Exec("SELECT pg_cancel_backend($1)", f.pid)
	if err != nil {
		return nil, err
	}
	err = <-notify
	if err != nil {
		return nil, err
	}
	_, err = f.txn.Exec("ROLLBACK TO SAVEPOINT s")
	if err != nil {
		return nil, err
	}

	return f.fetchData()
}

func (f *Fetcher) listener(notify chan error) {
	_, err := f.txn.Exec("SELECT pgcov.pgcov_listen()")
	if e, ok := err.(*pq.Error); !ok || e.Code.Name() != "query_canceled" {
		notify <- err
	}
	close(notify)
}

func (f *Fetcher) fetchData() (functions []*Function, err error) {
	rows, err := f.txn.Query("SELECT fnsignature, ncalls FROM pgcov.pgcov_called_functions() ORDER BY fnsignature")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	functions = nil
	for rows.Next() {
		fn := &Function{}
		err = rows.Scan(&fn.Signature, &fn.Calls)
		if err != nil {
			return nil, err
		}
		functions = append(functions, fn)
	}
	err = rows.Err()
	if err != nil {
		return nil, err
	}
	rows.Close()

	for _, fn := range(functions) {
		err = f.txn.QueryRow("SELECT pgcov.pgcov_fn_line_coverage_src($1)", fn.Signature).Scan(&fn.prosrc)
		if err != nil {
			return nil, err
		}

		rows, err = f.txn.Query("SELECT lineno, ncalls FROM pgcov.pgcov_fn_line_coverage($1) ORDER BY lineno", fn.Signature)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			line := sourceLine{}
			err = rows.Scan(&line.lineno, &line.ncalls)
			if err != nil {
				return nil, err
			}
			fn.sourceLines = append(fn.sourceLines, line)
		}
		err = rows.Err()
		if err != nil {
			return nil, err
		}
	}
	return functions, nil
}
