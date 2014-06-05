pgcov-html
==========

This is a trivial frontend for https://github.com/johto/pgcov

NOTE: If you want to run this on 9.1, you need to change the query in
fetcher.go to use "procpid" instead of "pid".  Or, better, submit a pull
request to figure out the version automatically and run the appropriate query
based on the server version.
