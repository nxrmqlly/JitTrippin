# spec

spec is a collection of Documentation and Devnotes

## Separation of Concerns

- #1 priority in jt codebase
- Abstractions over Lock-ins
- Easy to extend codebase
- `jtd` is NOT the engine, its one blessed REST API on top of the Engine
- `jt` is NOT the engine, its one blessed client for `jtd`
- The engine itself is exceptionally small
