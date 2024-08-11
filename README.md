<!-- PROJECT LOGO -->
<br />
<div align="center">
<h3 align="center">List</h3>
  <a href="https://github.com/arjungandhi/list">
    <img src="images/logo.png" alt="Logo" height="200">
  </a>

  <p align="center">
    money money money is so funny in a rich mans world 
  </p>
</div>

[![Go Report Card](https://goreportcard.com/badge/github.com/arjungandhi/list?style=flat-square)](https://goreportcard.com/report/github.com/arjungandhi/list)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/arjungandhi/list)](https://pkg.go.dev/github.com/arjungandhi/list)
[![Release](https://img.shields.io/github/release/arjungandhi/list.svg?style=flat-square)](https://github.com/arjungandhi/list/releases/latest)


# About
Money is a money cli that tracks personal finances with plaid

Built with [https://github.com/rwxrob/bonzai](https://github.com/rwxrob/bonzai)

## Install

This command can be installed as a standalone program or composed into a
Bonzai command tree.

Standalone

```
go install github.com/arjungandhi/money/cmd/money@latest
```

Composed

```go
package z

import (
	Z "github.com/rwxrob/bonzai/z"
	money "github.com/arjungandhi/money"
)

var Cmd = &Z.Cmd{
	Name:     `z`,
	Commands: []*Z.Cmd{help.Cmd, money.Cmd},
}
```

## Tab Completion

To activate bash completion just use the `complete -C` option from your
`.bashrc` or command line. There is no messy sourcing required. All the
completion is done by the program itself.

```
complete -C money money
```

If you don't have bash or tab completion check use the shortcut
commands instead.

## Embedded Documentation

All documentation (like manual pages) has been embedded into the source
code of the application. See the source or run the program with help to
access it.


