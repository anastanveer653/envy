# Renders docs/demo.gif with https://github.com/charmbracelet/vhs
#
#   go build -o envy . && vhs docs/demo.tape
#
# Run it from the repository root. The recording uses a throwaway directory.

Output docs/demo.gif
Require git

Set Shell bash
Set FontSize 18
Set Width 1100
Set Height 640
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 45ms

Hide
Type `export PATH="$PWD:$PATH" ENVY_PASSWORD=demo-passphrase-42 && cd "$(mktemp -d)" && git init -q && clear`
Enter
Show

Type "envy init"  Enter  Sleep 1.5s
Type "echo 'postgres://app:s3cret@db/prod' | envy set DATABASE_URL -e production"  Enter  Sleep 1s
Type "envy set SENTRY_DSN https://k@sentry.io/1 -e production"  Enter  Sleep 1s
Type "envy list -e production"  Enter  Sleep 2s
Type "envy run -e production -- sh -c 'echo connecting to $DATABASE_URL'"  Enter  Sleep 2s
Type "envy diff development production"  Enter  Sleep 2.5s
Type "echo 'X=1' > .env.local && envy audit"  Enter  Sleep 3s
