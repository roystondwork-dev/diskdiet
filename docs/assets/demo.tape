# Demo recording for the README, with made-up data (DISKDIET_DEMO).
# Render with: make demo  (requires https://github.com/charmbracelet/vhs)
Output docs/assets/demo.gif

Set Shell "bash"
Set FontSize 15
Set Width 1180
Set Height 700
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

Hide
Type "export PATH=$PWD/bin:$PATH DISKDIET_DEMO=1 PS1='$ '" Enter
Type "clear" Enter
Show

Type "diskdiet" Sleep 400ms Enter
Sleep 5s

Type "clear" Enter
Type "diskdiet clean --skip docker" Sleep 400ms Enter
Sleep 4s
Type "y" Sleep 300ms Enter
Sleep 5s
