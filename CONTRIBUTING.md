# Contributing

We welcome bug reports, feature suggestions and code contributions.

## Prerequisites

- [Go](https://go.dev/)
- [Task](https://taskfile.dev/)
- [pre-commit](https://pre-commit.com/)

## Quick start

1. Clone the repository.
2. Install hooks:
   ```bash
   pre-commit install
   ```

## Checks and tests

Common `task` commands used in the project:

| Command | Purpose |
|---------|---------|
| `task fmt` | Auto-fix code style |
| `task test-unit` | Run unit tests |
| `task test-integration` | Run integration tests |
| `task test-e2e` | Run e2e tests |
| `task test-golden-update` | Regenerate dashboard golden files after an intended UI change |
| `task mocks` | Regenerate testify mocks with mockery |
| `task license-headers-fix` | Add MPL-2.0 license headers |
| `task license-check` | Check dependency licenses |

Other commands can be viewed by running:

```shell
task
```

## Development

For quick debugging, you can spin up a local VM:

```bash
task dev:start-vm dev:provision
task dev:ssh -- machine info
```

## AI usage

AI tools may be used to help write code, but you are responsible for the result:

- Do not submit code you do not fully understand.
- Issues and pull request messages should be written by you. Using AI for
  grammar or spell checking (without significant changes) is fine, but do
  not generate entire messages with AI.

## License

All contributions are released under the [MPL-2.0](LICENSE) license.
