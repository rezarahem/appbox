# appbox

A small, dependency-free Go CLI for installing AppImages for the current user.
It copies the file to `~/.local/share/appbox`, creates an application-menu
entry, and extracts an icon if it can find one.

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
appbox list
appbox rm example-app
```

If no custom name is provided, it uses the AppImage filename. `rm` only removes
the AppImage, icon, and desktop entry created by appbox.
