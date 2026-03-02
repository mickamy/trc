Output docs/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 1400
Set Height 700
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

# Launch TUI with sample NDJSON data
Type "trc -i docs/demo.ndjson"
Sleep 500ms
Enter
Sleep 2s

# Navigate down through records
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "j"
Sleep 300ms
Type "j"
Sleep 500ms

# Navigate up
Type "k"
Sleep 300ms
Type "k"
Sleep 300ms
Type "k"
Sleep 500ms

# Show trace tree for the selected record
Enter
Sleep 3s

# Back to watch view
Escape
Sleep 1s

# Show service dependency map
Type "m"
Sleep 3s

# Back to watch view
Escape
Sleep 1s

# Quit
Type "q"
Sleep 500ms
