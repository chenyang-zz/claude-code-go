package repl

func IsSlashCommand(input string) bool {
	return len(input) > 0 && input[0] == '/'
}
