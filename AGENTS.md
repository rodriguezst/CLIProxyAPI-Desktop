# CLIProxyAPI Desktop

This is an independent repository. Bundle official CLIProxyAPI releases; do not
copy, patch, or import its server source. Keep all changes inside this repository.

Use Go 1.26+, English comments, and small platform adapters. Native tray imports
must be restricted to Windows/macOS so shared tests can run on Linux without GTK.
Never log credentials. Keep network timeouts confined to local startup probes and
release downloads; the wrapper does not configure upstream proxy connections.

After Go changes, run `gofmt -w .`, `go test ./...`, and build both
`./cmd/desktop` and `./cmd/supervisor`. After build-tool changes run
`python3 -m unittest discover -s scripts -p 'test_*.py'`.

Windows and macOS native builds and packaged smoke tests run in GitHub Actions.
Do not add a remote, push, or publish releases unless the user requests it.
