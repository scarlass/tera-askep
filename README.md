# Tera Askep
synchronize your form directly

## Requirement
- Go cli (>= v1.23.5 - for installation)
- SSH
- Psql cli

## Installation
```bash
go install github.com/scarlass/tera-askep@latest
```

## Usage
```bash
Usage:
  tera-askep [command]

Available Commands:
  help        Help about any command
  init        initialize sync configuration file
  sync        synchronize target project to askep_list table

Flags:
  -h, --help   help for tera-askep
```

more examples can be seen in [tera-askep-form](https://github.com/scarlass/tera-askep-form)
