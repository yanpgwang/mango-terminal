Output dist/mango-endpoint-demo.gif

Set Shell "zsh"
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Width 1440
Set Height 900
Set Theme "TokyoNightStorm"
Set TypingSpeed 35ms
Set Framerate 30
Set PlaybackSpeed 1

Env TERM "xterm-256color"
Env COLORTERM "truecolor"

Type "env -u NO_COLOR ./bin/mango --demo"
Enter
Sleep 3s

# Open the endpoint picker and browse the automatically discovered choices.
Enter
Sleep 2s
Down@350ms 2
Sleep 1s

# Show manual endpoint entry, then return without changing the demo target.
Type "e"
Sleep 1s
Type "https://mango.example.com"
Sleep 2s
Escape
Sleep 1s
Up@350ms 2
Enter
Sleep 1s

# The final Enter validates the selected endpoint and enters Mango.
Enter
Sleep 4s

Ctrl+C
Sleep 1s
Ctrl+C
Sleep 1s
