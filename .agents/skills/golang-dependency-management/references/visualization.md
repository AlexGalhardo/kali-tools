# Visualizing the Dependency Graph

## go mod graph (Built-in)

```bash
go mod graph
```

Output: each line contains two space-separated fields (module and its requirement) in `path@version` format:

```text
example.com/main github.com/google/uuid@v1.6.0
example.com/main golang.org/x/text@v0.3.7
github.com/google/uuid@v1.6.0 golang.org/x/sys@v0.0.0-20210615035016
```

## go mod why

```bash
go mod why -m github.com/some/module
```

Shows the shortest import path from your code to the module — useful for understanding why an unexpected dependency exists.

## Generate a Graph Image with modgraphviz

Run `modgraphviz` at a pinned version without adding it to `go.mod`:

```bash
go mod graph | go run golang.org/x/exp/cmd/modgraphviz@<pinned-version> > deps.dot
```

Green nodes represent versions selected by MVS (in the final build list). Grey nodes are versions that exist in the requirement graph but are not used.

## Complementary Analysis

For the architecture's package-level dependency rule (not modules), use `go list -deps` or depguard; see `golang-lint` and `docs/architecture-research.md` §11.
