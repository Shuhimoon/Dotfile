// Package dotfiles embeds the config files that `tools` deploys, so a
// released binary works without a checkout of this repo.
package dotfiles

import "embed"

// Configs holds starship/, zellij/ and the shells/ files used by
// configcopy.FilesFor, with the same layout as the repo.
//
//go:embed starship zellij shells/zsh/zshrc shells/starship
var Configs embed.FS
