package main

import "fmt"

// completionFlags is the flag list the completions offer. mermaid's flags are
// spelled with one dash by the Go flag package, but two dashes work too, so both
// forms are completed.
var completionFlags = []struct{ name, desc string }{
	{"w", "wrap width in columns"},
	{"theme", "colors.toml instead of the desktop theme"},
	{"plain", "print once instead of opening the viewer"},
	{"palette", "print the resolved palette and exit"},
	{"toc", "open the table of contents on start"},
	{"images", "inline images: auto, kitty, iterm2, sixel, halfblock, off"},
	{"config", "read another config file"},
	{"frame", "print one viewer frame and exit"},
	{"h", "frame height in rows"},
	{"find", "search for this text in the printed frame"},
	{"hscroll", "scroll the printed frame sideways"},
	{"man", "print the manual and exit"},
	{"version", "print the version and exit"},
	{"completions", "print a completion script and exit"},
}

const imageModes = "auto kitty iterm2 sixel halfblock off"

// completionScript returns the completion script for shell, one of bash, zsh or
// fish.
func completionScript(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashCompletion(), nil
	case "zsh":
		return zshCompletion(), nil
	case "fish":
		return fishCompletion(), nil
	}
	return "", fmt.Errorf("no completions for %q (bash, zsh, fish)", shell)
}

func bashCompletion() string {
	s := `# bash completion for mdv
# install: mdv -completions bash > /etc/bash_completion.d/mdv
_mdv() {
    local cur prev
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    case "$prev" in
        -images|--images)  COMPREPLY=( $(compgen -W "` + imageModes + `" -- "$cur") ); return ;;
        -completions|--completions) COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") ); return ;;
        -theme|--theme|-config|--config) COMPREPLY=( $(compgen -f -- "$cur") ); return ;;
        -w|--w|-h|--h|-hscroll|--hscroll) return ;;
    esac
    if [[ "$cur" == -* ]]; then
        COMPREPLY=( $(compgen -W "`
	for _, f := range completionFlags {
		s += "-" + f.name + " --" + f.name + " "
	}
	s += `" -- "$cur") )
        return
    fi
    COMPREPLY=( $(compgen -f -- "$cur") )
}
complete -o default -F _mdv mdv
`
	return s
}

func zshCompletion() string {
	s := `#compdef mdv
# install: mdv -completions zsh > "${fpath[1]}/_mdv"
_arguments -s \
`
	for _, f := range completionFlags {
		switch f.name {
		case "images":
			s += "  '(-images --images)'{-images,--images}'[inline images]:mode:(" + imageModes + ")' \\\n"
		case "completions":
			s += "  '(-completions --completions)'{-completions,--completions}'[print a completion script]:shell:(bash zsh fish)' \\\n"
		case "theme", "config":
			s += "  '(-" + f.name + " --" + f.name + ")'{-" + f.name + ",--" + f.name + "}'[" + f.desc + "]:file:_files' \\\n"
		case "w", "h", "hscroll", "find":
			s += "  '(-" + f.name + " --" + f.name + ")'{-" + f.name + ",--" + f.name + "}'[" + f.desc + "]:value:' \\\n"
		default:
			s += "  '(-" + f.name + " --" + f.name + ")'{-" + f.name + ",--" + f.name + "}'[" + f.desc + "]' \\\n"
		}
	}
	s += "  '*:file:_files'\n"
	return s
}

func fishCompletion() string {
	s := "# fish completion for mdv\n# install: mdv -completions fish > ~/.config/fish/completions/mdv.fish\n"
	for _, f := range completionFlags {
		line := "complete -c mdv -o " + f.name + " -l " + f.name + " -d '" + f.desc + "'"
		switch f.name {
		case "images":
			line += " -x -a '" + imageModes + "'"
		case "completions":
			line += " -x -a 'bash zsh fish'"
		case "theme", "config":
			line += " -r -F"
		case "w", "h", "hscroll", "find":
			line += " -x"
		}
		s += line + "\n"
	}
	s += "complete -c mdv -F\n"
	return s
}
