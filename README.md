# appbox

A small, dependency-free Go CLI for installing AppImages and Linux executables
for the current user. It copies the file to `~/.local/share/appbox` and creates
an application-menu entry. For AppImages, it also extracts an icon if it can
find one.

## Build and install

From this project folder, build the binary and add it to your user command
path:

```sh
mkdir -p bin ~/.local/bin
go build -o bin/appbox .
ln -sf "$(pwd)/bin/appbox" ~/.local/bin/appbox
```

Open a new terminal if `~/.local/bin` is not already on your `PATH`. You can
then run `appbox` from anywhere.

## Usage

```sh
appbox add ~/Downloads/Example.AppImage "Example App"
appbox add ./my-program "My Program"
appbox list
appbox rm my-program
```

If no custom name is provided, it uses the file name. AppImages are detected by
their `.AppImage` extension; other files are installed as executables and made
executable. `rm` removes the installed file, icon, and desktop entry created by
appbox.
