# Lessons

- Never put a literal credvault secret token (double-brace `SECRET:KEY` form) inside a Bash heredoc, e.g. when writing docs: the credvault PreToolUse hook resolves it and fails the whole command if the key is missing. Use the Write tool for files that must contain that string.
- vue-tsc 3.3.x does not support TypeScript 7; pin typescript@5.x in Vue projects until vue-tsc catches up.
- gofmt realigns struct field columns; exact-string Python replaces on struct literals silently miss. Use regex with `\s+` (or `re.sub` on the field name) when patching Go structs.
